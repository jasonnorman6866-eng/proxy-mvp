# proxy-mvp

This repository contains the v0.3 local relay milestone.

Scope is intentionally limited to a single owned test exit node and a local allowlisted target.

## What it includes

- `/health` endpoint
- Basic authentication
- CONNECT-only policy
- allowlisted host/port restriction
- exit-node registry
- authorized/healthy/enabled checks
- per-user connection limit
- metadata-only audit logging
- local relay to a controlled test egress node
- Docker support for quick local validation

## Local validation

Start target:

```bash
go run ./cmd/target
```

Start egress:

```bash
go run ./cmd/egress
```

Start gateway:

```bash
PROXY_USER=dev PROXY_PASSWORD=secret go run ./cmd/gateway
```

Health check:

```bash
curl http://localhost:8080/health
```

Proxy through the controlled local path:

```bash
curl --proxy-user dev:secret -x http://localhost:8080 http://localhost:9444
```

This should relay traffic only through the single controlled local exit node.

## Docker

```bash
docker compose up --build
```

## Important guardrail

This is not a public residential/mobile proxy platform. It is a local, explicit, authorized relay primitive for testing and governance before broader expansion.
