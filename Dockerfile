# syntax=docker/dockerfile:1.4

FROM alpine:3.22

LABEL org.opencontainers.image.authors="Nicolas Martyanoff"
LABEL org.opencontainers.image.url="https://github.com/galdor/boulevard"
LABEL org.opencontainers.image.documentation="https://github.com/galdor/boulevard"
LABEL org.opencontainers.image.source="https://github.com/galdor/boulevard"
LABEL org.opencontainers.image.licenses="ISC"
LABEL org.opencontainers.image.title="Boulevard"
LABEL org.opencontainers.image.description="Multi-purpose HTTP server and reverse proxy."

RUN <<EOF
    set -eu

    apk upgrade --no-cache
    apk add --no-cache curl
EOF

COPY bin/boulevard /usr/bin/boulevard
COPY bin/boulevard-cli /usr/bin/boulevard-cli
COPY cfg/docker.bcl /etc/boulevard/boulevard.bcl

HEALTHCHECK --start-period=5s --interval=1m --timeout=5s --retries=3 \
    CMD curl -I -f http://localhost:8080/status

CMD ["/usr/bin/boulevard", "-c", "/etc/boulevard/boulevard.bcl"]
