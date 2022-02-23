# syntax = docker/dockerfile:1
FROM golang:1.17-bullseye AS base
WORKDIR /go/src/app
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/root/.cache/go-build \
    go mod download -x
COPY . .

FROM base as tester
RUN --mount=type=cache,target=/root/.cache/go-build \
    go test ./... > test-results.txt 2>&1; exit 0

FROM scratch as test
COPY --from=tester /go/src/app/test-results.txt /

FROM base as builder
RUN --mount=type=cache,target=/root/.cache/go-build \
    go build

FROM scratch AS build
COPY --from=builder /go/src/app/yaegpe /

FROM debian:bullseye AS dist
ENV PROVIDER="https://cloudflare-eth.com"
EXPOSE 8080
RUN apt update && apt install -y ca-certificates
COPY --from=builder /go/src/app/yaegpe /usr/bin/
CMD yaegpe \
    -provider $PROVIDER