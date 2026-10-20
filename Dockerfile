FROM golang:1.22-alpine AS builder

WORKDIR /src
COPY go.mod ./
COPY internal ./internal
COPY cmd ./cmd
COPY config ./config
RUN go build -o /bin/gateway ./cmd/gateway

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /bin/gateway /bin/gateway
COPY --from=builder /src/config ./config
EXPOSE 8080
ENTRYPOINT ["/bin/gateway"]
