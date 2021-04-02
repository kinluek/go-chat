FROM golang:1.16.2 AS build-env

WORKDIR /backend
ADD . /backend

RUN go build -o /server cmd/server/main.go

FROM debian:buster

WORKDIR /
COPY --from=build-env /server /

ENTRYPOINT ["/server"]