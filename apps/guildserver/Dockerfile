FROM  --platform=${BUILDPLATFORM} golang:1.21-alpine AS build

# runs from project directory
WORKDIR /

COPY . .

RUN go mod download
RUN go build /apps/guildserver/cmd/guildserver/main.go

FROM alpine:3.19

WORKDIR /

COPY --from=build ./main ./guildserver

ENTRYPOINT ["./guildserver"]
