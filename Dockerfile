FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown

RUN CGO_ENABLED=0 go build \
    -ldflags "-s -w \
      -X github.com/org/github-runner/internal/version.Version=${VERSION} \
      -X github.com/org/github-runner/internal/version.Commit=${COMMIT} \
      -X github.com/org/github-runner/internal/version.Date=${DATE}" \
    -o /github-runner ./cmd/github-runner

FROM alpine:3.19

RUN apk add --no-cache \
    ca-certificates \
    git \
    bash \
    curl \
    docker-cli \
    && addgroup -S runner \
    && adduser -S runner -G runner

COPY --from=builder /github-runner /usr/local/bin/github-runner

RUN mkdir -p /etc/github-runner /var/lib/github-runner /var/log/github-runner \
    && chown -R runner:runner /var/lib/github-runner /var/log/github-runner

USER runner

VOLUME ["/var/lib/github-runner"]

ENTRYPOINT ["github-runner"]
CMD ["start", "--config", "/etc/github-runner/config.toml"]
