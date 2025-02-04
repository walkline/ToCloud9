# syntax = docker/dockerfile:1-experimental
FROM --platform=${BUILDPLATFORM} golang:1.23-alpine AS build

ARG TARGETOS
ARG TARGETARCH

# runs from project directory
WORKDIR /

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-w -s" /apps/matchmakingserver/cmd/matchmakingserver/main.go

FROM alpine:3.19

WORKDIR /

COPY --from=build ./main ./matchmakingserver

EXPOSE 8080

ENTRYPOINT ["./matchmakingserver"]
