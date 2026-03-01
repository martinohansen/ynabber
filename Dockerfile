# Build stage
FROM --platform=$BUILDPLATFORM golang:alpine AS builder
RUN apk add --no-cache git
WORKDIR /go/src/app
COPY . .

ARG TARGETOS TARGETARCH
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH && \
    go get -d -v ./... && \
    go build -o /go/bin/app -v ./cmd/ynabber/.

# Final stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates && \
    adduser -D -u 1000 ynabber
COPY --from=builder /go/bin/app /app
USER ynabber
ENTRYPOINT ["/app"]
