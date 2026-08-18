# Build stage
FROM --platform=$BUILDPLATFORM golang:alpine AS builder
RUN apk add --no-cache git
WORKDIR /go/src/app
COPY . .

ARG TARGETOS TARGETARCH TARGETVARIANT
RUN set -eux; \
    export CGO_ENABLED=0 GOOS="$TARGETOS" GOARCH="$TARGETARCH"; \
    target_goarm="${TARGETVARIANT#v}"; \
    if [ "$TARGETARCH" = "arm" ] && [ -n "$target_goarm" ]; then \
        export GOARM="$target_goarm"; \
    fi; \
    go mod download; \
    go build -o /go/bin/app -v ./cmd/ynabber/.

# Final stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates && \
    adduser -D -u 1000 ynabber
COPY --from=builder /go/bin/app /app
USER ynabber
ENTRYPOINT ["/app"]
