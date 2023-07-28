FROM  --platform=${BUILDPLATFORM} golang:1.19-alpine AS build

# runs from project directory
WORKDIR /

COPY . .

RUN go mod download
RUN go build /apps/chatserver/cmd/chatserver/main.go

FROM alpine:3.14

WORKDIR /

COPY --from=build ./main ./chatserver

ENTRYPOINT ["./chatserver"]
