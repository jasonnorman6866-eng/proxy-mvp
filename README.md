# proxy-mvp

This is a minimal, audited, explicit-node proxy gateway that proves the path:

client -> authenticated gateway -> one authorized exit node -> public site

This is intentionally not a commercial or residential-proxy platform. It is a tiny proof of concept.

## What it does

- HTTP CONNECT proxying
- Basic authentication
- Single configured exit node
- Destination and port restrictions
- Rate limiting
- Connection/bandwidth counting
- Audit logging to stdout
- Kill switch via environment variable

## Run locally

Start the exit node:

```bash
go run ./cmd/node
```

Start the gateway:

```bash
GATEWAY_USERNAME=demo \
GATEWAY_PASSWORD=demo \
GATEWAY_EXIT_NODE_HOST=127.0.0.1 \
GATEWAY_EXIT_NODE_PORT=9001 \
GATEWAY_ALLOWED_HOSTS=example.com \
GATEWAY_ALLOWED_PORTS=443 \
GATEWAY_RATE_LIMIT=60 \
GATEWAY_RATE_BURST=30 \
go run ./cmd/gateway
```

Test:

```bash
curl --proxy-user demo:demo -x http://localhost:8080 https://example.com -I
```

If the tunnel works, the HTTP response should come back from `example.com`.

## Docker

```bash
docker compose up --build
```

Then:

```bash
curl --proxy-user demo:demo -x http://localhost:8080 https://example.com -I
```

## Governance note

This MVP assumes:
- explicit owner/authorized use
- one controlled exit node
- no peer-to-peer SDK or arbitrary residential network
- explicit destination restrictions and audit logging

This keeps the initial architecture aligned with consent and abuse controls.
