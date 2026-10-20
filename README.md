# proxy-mvp

This repository is the first small, controlled gateway proof of concept for a legitimate single-node egress path.

The design is intentionally narrow:

- one configured exit node
- basic authentication
- CONNECT tunneling
- restrictive policy
- rate limiting
- usage accounting
- audit logs without payload capture
- health endpoint and kill switch

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

Then verify:

```bash
curl --proxy-user demo:demo -x http://localhost:8080 https://example.com -I
```

Health check:

```bash
curl http://localhost:8080/health
```

## Containerized

```bash
docker compose up --build
```

## Governance note

This is the first controlled primitive: a gateway with a single authorized egress path, explicit deny/allow boundaries, and logging that tracks metadata and bytes transferred without encoding or capturing payload contents.

It does not include residential SDK networks, carrier rotation, billing, or customer dashboards.
