# syntax = docker/dockerfile:1-experimental
FROM --platform=${BUILDPLATFORM} golang:1.21-alpine AS build

ARG TARGETOS
ARG TARGETARCH

# runs from project directory
WORKDIR /

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-w -s" /apps/servers-registry/cmd/servers-registry/main.go

FROM alpine:3.19

WORKDIR /

COPY --from=build ./main ./servers-registry

EXPOSE 8080

ENTRYPOINT ["./servers-registry"]
