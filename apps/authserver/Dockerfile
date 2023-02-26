FROM golang:1.19-alpine AS build

# runs from project directory
WORKDIR /

COPY . .

RUN go mod download

RUN go build /apps/authserver/cmd/authserver/main.go

FROM alpine:3.14

WORKDIR /

COPY --from=build ./main ./authserver

EXPOSE 8080

ENTRYPOINT ["./authserver"]
