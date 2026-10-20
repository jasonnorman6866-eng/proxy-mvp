FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN go build -o /bin/gateway ./cmd/gateway
RUN go build -o /bin/egress ./cmd/egress
RUN go build -o /bin/target ./cmd/target

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /bin/gateway /bin/gateway
COPY --from=builder /bin/egress /bin/egress
COPY --from=builder /bin/target /bin/target
EXPOSE 8080 9443 9444
ENTRYPOINT ["/bin/gateway"]
