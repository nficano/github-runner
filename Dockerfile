FROM alpine:3.19

RUN apk add --no-cache \
    ca-certificates \
    git \
    bash \
    curl \
    docker-cli \
    && addgroup -S runner \
    && adduser -S runner -G runner

COPY github-runner /usr/local/bin/github-runner

RUN mkdir -p /etc/github-runner /var/lib/github-runner /var/log/github-runner \
    && chown -R runner:runner /var/lib/github-runner /var/log/github-runner

USER runner

VOLUME ["/var/lib/github-runner"]

ENTRYPOINT ["github-runner"]
CMD ["start", "--config", "/etc/github-runner/config.toml"]
