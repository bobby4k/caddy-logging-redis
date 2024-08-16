# Redis logger module for Caddy

This plugin sends Caddy logs to Redis using the LPUSH command, allowing other consumers to process the logs.

- It helps address issues related to centralized log collection and the I/O and storage challenges of keeping logs locally.
- It's particularly suited for **small-scale operations deployed on VPS**.
- In the future, if scaling is needed, you can consider using Redis Cluster or DragonflyDB.

通过本插件，可把caddy日志通过LPUSH发送给redis, 以供其他消费端处理。
- 它可以解决日志收集中心化、本地日志存储带来的io和容量问题。
- 比较适合部署在**VPS上的小规模业务**。
- 未来如果要扩容，可以考虑Redis Cluster或Dragonflydb

## Configuration

### Simple mode

Enable Redis logger for Caddy by specifying the module configuration in the Caddyfile:
```
{
    order redis_logger after log
}

:80 {
    route {
        redis_logger my_redis_key {
            redis_address localhost:6379
            redis_password mypassword
            with_request_body
        }
        respond "Hello, World!"
    }
}

```
Note that `address` and `db` values can be configured (or accept the defaults) .

The module supports [environment variable substitution](https://caddyserver.com/docs/caddyfile/concepts#environment-variables) within Caddyfile parameters:
```
{
    redis_logger my_redis_key {
        redis_address        "{$REDIS_ADDRESS}"
        redis_password       "{$REDIS_PASSWORD}"
    }
}
```

NOTE however the following configuration options also support runtime substition:

- redis_db          // default 0
- dial_timeout      // 连接超时时间 default 5s
- write_timeout     // 写入超时时间 default 3s
- max_retries       // 最大重试次数 default 3

### Not support
- Redis Cluster
- Failover mode
- TLS


