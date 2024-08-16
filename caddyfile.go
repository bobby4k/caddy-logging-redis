package redislogger

import (
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

func init() {
	caddy.RegisterModule(RedisLogger{})
	httpcaddyfile.RegisterHandlerDirective("redis_logger", parseCaddyfile)
}

func (RedisLogger) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.redis_logger",
		New: func() caddy.Module { return new(RedisLogger) },
	}
}

// UnmarshalCaddyfile实现了caddyfile.Unmarshaler
func (rl *RedisLogger) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		if !d.Args(&rl.RedisKey) {
			return d.Err("missing Redis key")
		}

		for d.NextBlock(0) {
			switch d.Val() {
			case "with_request_body":
				rl.WithBody = true
			case "redis_address":
				if !d.Args(&rl.RedisAddress) {
					return d.Err("missing Redis address")
				}
			case "redis_password":
				if !d.Args(&rl.RedisPassword) {
					return d.Err("missing Redis password")
				}
			}
		}
	}
	return nil
}

// parseCaddyfile从h中解读令牌到一个新的中间件。
func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	var rl RedisLogger
	err := rl.UnmarshalCaddyfile(h.Dispenser)
	return &rl, err
}

// Interface guards
var (
	_ caddy.Provisioner           = (*RedisLogger)(nil)
	_ caddy.Validator             = (*RedisLogger)(nil)
	_ caddyhttp.MiddlewareHandler = (*RedisLogger)(nil)
	_ caddyfile.Unmarshaler       = (*RedisLogger)(nil)
)
