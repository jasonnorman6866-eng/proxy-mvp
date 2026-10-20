# proxy-mvp

This repository is the first controlled milestone for a local, allowlisted Gateway v0.1.

It intentionally proves the authorization and policy gates before enabling a real egress tunnel.

## What this version does

- `/health` endpoint
- Basic auth for proxy requests
- CONNECT-only handling
- allowlist for `example.com`
- port restriction to `443`
- deny unauthorized destinations
- explicit `501 Not Implemented` while the relay is intentionally disabled

## Run locally

```bash
PROXY_USER=dev \
PROXY_PASSWORD=test-password \
go run ./cmd/gateway
```

Health check:

```bash
curl http://localhost:8080/health
```

Authenticated proxy test:

```bash
curl \
  -x http://dev:test-password@localhost:8080 \
  https://example.com
```

This should authenticate and authorize the destination, but stop before the actual TCP relay is enabled.

Unauthorized destination example:

```bash
curl \
  -x http://dev:test-password@localhost:8080 \
  https://www.google.com
```

This should be rejected.

## Why this is the right first checkpoint

This keeps the initial build narrow and safe:
- no arbitrary residential/mobile network
- no open proxy behavior
- no real egress routing yet
- explicit allowlist and deny states

The next checkpoint is an operator-controlled exit-node registry with health + kill switch + relay path.
