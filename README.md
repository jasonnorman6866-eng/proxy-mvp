# proxy-mvp

This repository contains the v0.3 milestone: the gateway keeps the same auth/policy/node-safety gates as v0.2, but now adds a controlled relay path to a local operator-owned test environment.

The relay is intentionally restrictively scoped:

- one explicit exit node
- local test target only
- local egress service required
- no arbitrary public proxying
- no peer residential/mobile network
- no customer billing or dashboard

## Components

- `cmd/gateway` — authenticated gateway with policy and node checks
- `cmd/egress` — local operator-controlled relay endpoint on `:9443`
- `cmd/target` — local echo target on `:9444` for controlled testing

## Local validation

Start the controlled test environment:

```bash
go run ./cmd/target &
go run ./cmd/egress &
```

Then start the gateway:

```bash
PROXY_USER=dev \
PROXY_PASSWORD=secret \
go run ./cmd/gateway
```

Health check:

```bash
curl http://localhost:8080/health
```

Test the local controlled relay:

```bash
curl \
  -x http://dev:secret@localhost:8080 \
  http://localhost:9444
```

If the tunnel is working, the body should echo back through the gateway and the local egress path.

## Docker

```bash
docker compose up --build
```

## Definition of done

This milestone proves the following sequence:

- auth passes
- policy passes
- node eligibility passes
- relay to a single controlled local exit node passes
- byte traffic is observed through the tunnel
- the relay is still restricted to an explicit, operator-controlled path
