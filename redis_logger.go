package redislogger

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

func init() {
	caddy.RegisterModule(RedisLogger{})
}

type RedisLogger struct {
	RedisAddress  string `json:"redis_address"`
	RedisPassword string `json:"redis_password"`
	RedisKey      string `json:"redis_key"`
	WithBody      bool   `json:"with_body,omitempty"`
	client        *redis.Client
	logger        *zap.Logger
}

func (RedisLogger) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.redis_logger",
		New: func() caddy.Module { return new(RedisLogger) },
	}
}

func (rl *RedisLogger) Provision(ctx caddy.Context) error {
	rl.logger = ctx.Logger(rl)

	rl.client = redis.NewClient(&redis.Options{
		Addr:     rl.RedisAddress,
		Password: rl.RedisPassword,
	})

	return nil
}

func (rl *RedisLogger) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	recorder := caddyhttp.NewResponseRecorder(w, nil, nil)

	if err := next.ServeHTTP(recorder, r); err != nil {
		return err
	}

	logEntry := map[string]interface{}{
		"method":      r.Method,
		"uri":         r.RequestURI,
		"status":      recorder.Status(),
		"remote_addr": r.RemoteAddr,
	}

	if rl.WithBody {
		body, err := ioutil.ReadAll(r.Body)
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
	}

	return nil
}

func (rl *RedisLogger) Cleanup() error {
	return rl.client.Close()
}
