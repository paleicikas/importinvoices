# syntax=docker/dockerfile:1

# build stage
FROM golang:1.26-alpine AS build
WORKDIR /src

# Cache module downloads independently of source changes.
COPY server/go.mod server/go.sum ./server/
RUN cd server && go mod download

COPY server/ ./server/
RUN cd server && \
    CGO_ENABLED=0 go build \
        -trimpath \
        -ldflags="-s -w" \
        -o /out/importinvoices ./cmd/importinvoices

# runtime stage
FROM alpine:3.20

# ca-certificates: required for HTTPS calls to OpenAI / Gemini.
# tzdata: required for proper time-zone handling in logs and timestamps.
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 10001 app

COPY docker/entrypoint.sh /usr/local/bin/entrypoint.sh
COPY --from=build /out/importinvoices /usr/local/bin/importinvoices

RUN mkdir -p /data && \
    chown -R app:app /data /usr/local/bin/entrypoint.sh /usr/local/bin/importinvoices && \
    chmod +x /usr/local/bin/entrypoint.sh

USER app
ENV DATA_DIR=/data
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["serve"]
