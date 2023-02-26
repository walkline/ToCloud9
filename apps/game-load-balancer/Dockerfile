# syntax = docker/dockerfile:1-experimental
FROM --platform=${BUILDPLATFORM} golang:1.19-alpine AS build

# runs from project directory
WORKDIR /

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN --mount=type=cache,target=/root/.cache/go-build \
    go build /apps/game-load-balancer/cmd/game-load-balancer/main.go

FROM alpine:3.14

WORKDIR /

COPY --from=build ./main ./game-load-balancer

EXPOSE 8080

ENTRYPOINT ["./game-load-balancer"]
