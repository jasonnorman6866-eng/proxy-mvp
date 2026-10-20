# proxy-mvp

This repository is the first small, controlled Gateway v0.1 milestone for a legitimate single-node egress path.

The build is intentionally narrow:

- Go `net/http` server
- authenticated CONNECT proxy
- deny/allow policy for destination ports
- explicit egress enablement
- user-level connection cap
- rate limiting and usage tracking
- audit logging for metadata only
- no payload capture
- /health endpoint

## Project structure

```text
proxy-mvp/
├── cmd/
│   └── gateway/
│       └── main.go
├── internal/
│   ├── audit/
│   │   └── audit.go
│   ├── auth/
│   │   └── auth.go
│   ├── meter/
│   │   └── meter.go
│   ├── policy/
│   │   └── policy.go
│   └── proxy/
│       └── connect.go
├── config/
│   └── config.yaml
├── Dockerfile
├── docker-compose.yml
├── go.mod
├── README.md
└── .gitignore
```

## Run locally

```bash
go run ./cmd/gateway
```

Check health:

```bash
curl http://localhost:8080/health
```

Expected response:

```json
{"status":"ok"}
```

Proxy test:

```bash
curl --proxy-user demo:demo -x http://localhost:8080 https://example.com -I
```

## Notes

This milestone intentionally keeps routing and egress to a known, authorized, operator-controlled path only. It does not support residential SDK networks, peer rotation, or carrier impersonation.

## Definition of done

The service should authenticate, enforce the restrictive port policy, relay the CONNECT tunnel, and emit metadata events without logging payloads or TLS contents.
