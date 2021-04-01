  
FROM golang:latest

LABEL maintainer="linhnln"

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

EXPOSE 1886

RUN cd mainflux-master/cmd/mqtt && go run main.go