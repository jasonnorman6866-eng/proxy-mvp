# proxy-mvp

This repository is the first small, controlled gateway proof of concept for a legitimate single-node egress path.

It intentionally does not include residential SDK networks, billing, dashboards, or carrier rotation.

## Scope

The MVP proves this path:

```text
Client
  ↓
Go Gateway :8080
  ↓
Authentication
  ↓
Policy check
  ↓
Authorized exit node
  ↓
Public destination
```

## Included

- HTTP CONNECT proxying
- Basic authentication
- Restrictive destination and port policy
- One configured exit node
- Rate limiting
- Bandwidth tracking
- Audit logging without payload capture
- /health endpoint
- kill switch flag

## Project structure

```text
proxy-mvp/
├── cmd/
│   └── gateway/
│       └── main.go
├── internal/
│   ├── audit/
│   ├── auth/
│   ├── config/
│   ├── meter/
│   ├── policy/
│   └── proxy/
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

Then test with:

```bash
curl --proxy-user demo:demo -x http://localhost:8080 https://example.com -I
```

Check health:

```bash
curl http://localhost:8080/health
```

## Docker

```bash
docker compose up --build
```

## Governance notes

This gateway assumes a single known, authorized egress point and a strict allow list. It does not impersonate carriers, rotate IPs, or rely on arbitrary residential traffic.

## Definition of done

This repo proves the minimal foundation required before expanding into PostgreSQL, node registry, and routing.
