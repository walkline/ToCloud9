FROM  --platform=${BUILDPLATFORM} golang:1.21-alpine AS build

ARG TARGETOS
ARG TARGETARCH

# runs from project directory
WORKDIR /

COPY . .

RUN go mod download
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-w -s" /apps/charserver/cmd/charserver/main.go

FROM alpine:3.19

WORKDIR /

COPY --from=build ./main ./charserver

EXPOSE 8080

ENTRYPOINT ["./charserver"]
