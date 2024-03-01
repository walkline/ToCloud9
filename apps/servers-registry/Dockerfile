# syntax = docker/dockerfile:1-experimental
FROM --platform=${BUILDPLATFORM} golang:1.21-alpine AS build

# runs from project directory
WORKDIR /

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build /apps/servers-registry/cmd/servers-registry/main.go

FROM alpine:3.19

WORKDIR /

COPY --from=build ./main ./servers-registry

EXPOSE 8080

ENTRYPOINT ["./servers-registry"]
