FROM --platform=$BUILDPLATFORM golang:1.22 AS builder

WORKDIR /workspace

ARG TARGETOS=linux
ARG TARGETARCH

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /out/kube-sentinel ./cmd/manager

FROM alpine:3.20

RUN addgroup -S kube-sentinel && adduser -S kube-sentinel -G kube-sentinel

WORKDIR /home/kube-sentinel
COPY --from=builder /out/kube-sentinel /usr/local/bin/kube-sentinel

USER kube-sentinel

ENTRYPOINT ["/usr/local/bin/kube-sentinel"]