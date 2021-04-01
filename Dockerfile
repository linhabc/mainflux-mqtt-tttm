  
FROM golang:latest

LABEL maintainer="linhnln"

WORKDIR /app

COPY . .

RUN go run app/mainflux-master\cmd\mqt