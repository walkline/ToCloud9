FROM  --platform=${BUILDPLATFORM} golang:1.19-alpine AS build

# runs from project directory
WORKDIR /

COPY . .

RUN go mod download
RUN go build /apps/groupserver/cmd/groupserver/main.go

FROM alpine:3.14

WORKDIR /

COPY --from=build ./main ./groupserver

ENTRYPOINT ["./groupserver"]