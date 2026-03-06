FROM golang:1.22 AS builder

WORKDIR /workspace

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/kube-sentinel ./cmd/manager

FROM alpine:3.20

RUN addgroup -S kube-sentinel && adduser -S kube-sentinel -G kube-sentinel

WORKDIR /home/kube-sentinel
COPY --from=builder /out/kube-sentinel /usr/local/bin/kube-sentinel

USER kube-sentinel

ENTRYPOINT ["/usr/local/bin/kube-sentinel"]