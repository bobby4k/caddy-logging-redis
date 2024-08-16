package redislogger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

type RedisLogger struct {
	RedisAddress  string        `json:"redis_address,omitempty"`
	RedisPassword string        `json:"redis_password,omitempty"`
	RedisDB       int           `json:"redis_db,omitempty"`
	RedisKey      string        `json:"redis_key"`
	WithBody      bool          `json:"with_body,omitempty"`
	DialTimeout   time.Duration `json:"dial_timeout,omitempty"`  // 连接超时时间
	ReadTimeout   time.Duration `json:"read_timeout,omitempty"`  // 读取超时时间
	WriteTimeout  time.Duration `json:"write_timeout,omitempty"` // 写入超时时间
	MaxRetries    int           `json:"max_retries,omitempty"`   // 最大重试次数
	client        *redis.Client
	logger        *zap.Logger
}

// Provision实现了caddy.Provisioner
func (rl *RedisLogger) Provision(ctx caddy.Context) error {
	rl.logger = ctx.Logger(rl)

	// 设置默认配置
	if rl.RedisAddress == "" {
		rl.RedisAddress = "localhost:6379"
	}
	if rl.RedisDB == 0 {
		rl.RedisDB = 0
	}
	if rl.DialTimeout == 0 {
		rl.DialTimeout = 5 * time.Second // 默认连接超时时间
	}
	if rl.ReadTimeout == 0 {
		rl.ReadTimeout = 3 * time.Second // 默认读取超时时间
	}
	if rl.WriteTimeout == 0 {
		rl.WriteTimeout = 3 * time.Second // 默认写入超时时间
	}
	if rl.MaxRetries == 0 {
		rl.MaxRetries = 3 // 默认最大重试次数
	}

	rl.client = redis.NewClient(&redis.Options{
		Addr:         rl.RedisAddress,
		Password:     rl.RedisPassword,
		DB:           rl.RedisDB,
		DialTimeout:  rl.DialTimeout,
		ReadTimeout:  rl.ReadTimeout,
		WriteTimeout: rl.WriteTimeout,
		MaxRetries:   rl.MaxRetries,
	})

	// Use context for the Ping command
	// ctx := context.Background()
	_, err := rl.client.Ping(ctx).Result()
	if err != nil {
		rl.logger.Error("Failed to connect to Redis", zap.Error(err))
		return fmt.Errorf("could not connect to Redis: %w", err)
	}

	rl.logger.Info("Successfully connected to Redis",
		zap.String("redis_key", rl.RedisKey),
		zap.String("redis_address", rl.RedisAddress),
	)
	return nil
}

// Validate实现了caddy.Validator
func (rl *RedisLogger) Validate() error {
	if rl.client == nil {
		return fmt.Errorf("no redis connet")
	}
	return nil
}

// ServeHTTP 实现了 caddyhttp.MiddlewareHandler
func (rl *RedisLogger) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	start := time.Now()
	recorder := caddyhttp.NewResponseRecorder(w, nil, nil)

	if err := next.ServeHTTP(recorder, r); err != nil {
		rl.logger.Error("Error next ServeHTTP", zap.Error(err))
		return err
	}

	duration := time.Since(start).Seconds()
	logEntry := map[string]interface{}{
		// "level":  "info",
		"ts": time.Now().Format(time.RFC3339Nano),
		// "logger": "http.log.access.log0",
		// "msg":    "handled request",
		"request": map[string]interface{}{
			"remote_ip":   r.RemoteAddr,
			"remote_port": r.URL.Port(),
			"client_ip":   r.Header.Get("X-Forwarded-For"),
			"proto":       r.Proto,
			"method":      r.Method,
			"host":        r.Host,
			"uri":         r.RequestURI,
			"headers":     r.Header,
			"tls": map[string]interface{}{
				"resumed":      r.TLS.DidResume,
				"version":      r.TLS.Version,
				"cipher_suite": r.TLS.CipherSuite,
				"proto":        r.TLS.NegotiatedProtocol,
				"server_name":  r.TLS.ServerName,
			},
		},
		"bytes_read": r.ContentLength,
		// "user_id":      "", // 可以根据需求设置用户ID
		"duration":     duration,
		"size":         recorder.Size(),
		"status":       recorder.Status(),
		"resp_headers": recorder.Header(),
	}

	if rl.WithBody {
		// https://github.com/caddyserver/caddy/commit/6f0f159ba56adeb6e2cbbb408651419b87f20856
		body, err := io.ReadAll(r.Body)
		if err != nil {
			rl.logger.Error("Error reading request body", zap.Error(err))
			return err
		}
		logEntry["request_body"] = string(body)
	}

	logJSON, err := json.Marshal(logEntry)
	if err != nil {
		rl.logger.Error("Error marshaling log entry to JSON", zap.Error(err))
		return err
	}

	ctx := context.Background()
	if err := rl.client.LPush(ctx, rl.RedisKey, logJSON).Err(); err != nil {
		rl.logger.Error("Error pushing log entry to Redis", zap.Error(err))
	} else { //!TEST
		rl.logger.Info("Successfully pushed log entry to Redis", zap.String("key", rl.RedisKey))
	}

	return nil
}

func (rl *RedisLogger) Cleanup() error {
	return rl.client.Close()
}
