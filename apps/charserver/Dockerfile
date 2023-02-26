FROM golang:1.19-alpine AS build

# runs from project directory
WORKDIR /

COPY . .

RUN go mod download
RUN go build /apps/charserver/cmd/charserver/main.go

FROM alpine:3.14

WORKDIR /

COPY --from=build ./main ./charserver

EXPOSE 8080

ENTRYPOINT ["./charserver"]
