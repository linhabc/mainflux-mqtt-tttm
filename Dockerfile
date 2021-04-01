  
FROM golang:latest

LABEL maintainer="linhnln"

WORKDIR /app

COPY . .

RUN cd mainflux-master/cmd/mqtt && go run main.go