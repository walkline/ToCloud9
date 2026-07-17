# syntax = docker/dockerfile:1-experimental
FROM --platform=${BUILDPLATFORM} golang:1.23-alpine AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-w -s" /apps/layer-coordinator/cmd/layer-coordinator/main.go

FROM alpine:3.19
WORKDIR /
COPY --from=build ./main ./layer-coordinator
EXPOSE 8996
ENTRYPOINT ["./layer-coordinator"]
