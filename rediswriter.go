package logging

import (
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
)

func init() {
	caddy.RegisterModule(RedisWriter{})
}

// NetWriter implements a log writer that outputs to a network socket. If
// the socket goes down, it will dump logs to stderr while it attempts to
// reconnect.
type RedisWriter struct {
	// The address of the network socket to which to connect.
	Address string `json:"address,omitempty"`

	// The timeout to wait while connecting to the socket.
	DialTimeout caddy.Duration `json:"dial_timeout,omitempty"`

	// If enabled, allow connections errors when first opening the
	// writer. The error and subsequent log entries will be reported
	// to stderr instead until a connection can be re-established.
	SoftStart bool `json:"soft_start,omitempty"`

	addr caddy.NetworkAddress
}

// CaddyModule returns the Caddy module information.
func (RedisWriter) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "caddy.logging.writers.redislogger",
		New: func() caddy.Module { return new(RedisWriter) },
	}
}

// Provision sets up the module.
func (nw *RedisWriter) Provision(ctx caddy.Context) error {
	repl := caddy.NewReplacer()
	address, err := repl.ReplaceOrErr(nw.Address, true, true)
	if err != nil {
		return fmt.Errorf("invalid host in address: %v", err)
	}

	nw.addr, err = caddy.ParseNetworkAddress(address)
	if err != nil {
		return fmt.Errorf("parsing network address '%s': %v", address, err)
	}

	if nw.addr.PortRangeSize() != 1 {
		return fmt.Errorf("multiple ports not supported")
	}

	if nw.DialTimeout < 0 {
		return fmt.Errorf("timeout cannot be less than 0")
	}

	return nil
}

func (nw RedisWriter) String() string {
	return nw.addr.String()
}

// WriterKey returns a unique key representing this nw.
func (nw RedisWriter) WriterKey() string {
	return nw.addr.String()
}

// OpenWriter opens a new network connection.
func (nw RedisWriter) OpenWriter() (io.WriteCloser, error) {
	reconn := &RedisConn{
		nw:      nw,
		timeout: time.Duration(nw.DialTimeout),
	}
	conn, err := reconn.dial()
	if err != nil {
		if !nw.SoftStart {
			return nil, err
		}
		// don't block config load if remote is down or some other external problem;
		// we can dump logs to stderr for now (see issue #5520)
		fmt.Fprintf(os.Stderr, "[ERROR] net log writer failed to connect: %v (will retry connection and print errors here in the meantime)\n", err)
	}
	reconn.connMu.Lock()
	reconn.Conn = conn
	reconn.connMu.Unlock()
	return reconn, nil
}

// UnmarshalCaddyfile sets up the handler from Caddyfile tokens. Syntax:
//
//	net <address> {
//	    dial_timeout <duration>
//	    soft_start
//	}
func (nw *RedisWriter) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	d.Next() // consume writer name
	if !d.NextArg() {
		return d.ArgErr()
	}
	nw.Address = d.Val()
	if d.NextArg() {
		return d.ArgErr()
	}
	for d.NextBlock(0) {
		switch d.Val() {
		case "dial_timeout":
			if !d.NextArg() {
				return d.ArgErr()
			}
			timeout, err := caddy.ParseDuration(d.Val())
			if err != nil {
				return d.Errf("invalid duration: %s", d.Val())
			}
			if d.NextArg() {
				return d.ArgErr()
			}
			nw.DialTimeout = caddy.Duration(timeout)

		case "soft_start":
			if d.NextArg() {
				return d.ArgErr()
			}
			nw.SoftStart = true
		}
	}
	return nil
}

// redialerConn wraps an underlying Conn so that if any
// writes fail, the connection is redialed and the write
// is retried.
type RedisConn struct {
	net.Conn
	connMu     sync.RWMutex
	nw         RedisWriter
	timeout    time.Duration
	lastRedial time.Time
}

// Write wraps the underlying Conn.Write method, but if that fails,
// it will re-dial the connection anew and try writing again.
func (reconn *RedisConn) Write(b []byte) (n int, err error) {
	reconn.connMu.RLock()
	conn := reconn.Conn
	reconn.connMu.RUnlock()
	if conn != nil {
		if n, err = conn.Write(b); err == nil {
			return
		}
	}

	// problem with the connection - lock it and try to fix it
	reconn.connMu.Lock()
	defer reconn.connMu.Unlock()

	// if multiple concurrent writes failed on the same broken conn, then
	// one of them might have already re-dialed by now; try writing again
	if reconn.Conn != nil {
		if n, err = reconn.Conn.Write(b); err == nil {
			return
		}
	}

	// there's still a problem, so try to re-attempt dialing the socket
	// if some time has passed in which the issue could have potentially
	// been resolved - we don't want to block at every single log
	// emission (!) - see discussion in #4111
	if time.Since(reconn.lastRedial) > 10*time.Second {
		reconn.lastRedial = time.Now()
		conn2, err2 := reconn.dial()
		if err2 != nil {
			// logger socket still offline; instead of discarding the log, dump it to stderr
			os.Stderr.Write(b)
			return
		}
		if n, err = conn2.Write(b); err == nil {
			if reconn.Conn != nil {
				reconn.Conn.Close()
			}
			reconn.Conn = conn2
		}
	} else {
		// last redial attempt was too recent; just dump to stderr for now
		os.Stderr.Write(b)
	}

	return
}

func (reconn *RedisConn) dial() (net.Conn, error) {
	return net.DialTimeout(reconn.nw.addr.Network, reconn.nw.addr.JoinHostPort(0), reconn.timeout)
}

// Interface guards
var (
	_ caddy.Provisioner     = (*RedisWriter)(nil)
	_ caddy.WriterOpener    = (*RedisWriter)(nil)
	_ caddyfile.Unmarshaler = (*RedisWriter)(nil)
)
