package redislogger

import (
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
)

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
