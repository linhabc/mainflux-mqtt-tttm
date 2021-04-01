  
FROM golang:latest

LABEL maintainer="linhnln"

WORKDIR /app

COPY . .

RUN go run ./mainflux-master/cmd/mqtt/main.go