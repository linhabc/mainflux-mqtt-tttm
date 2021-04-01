  
FROM golang:latest

LABEL maintainer="linhnln"

WORKDIR /app
COPY ./mainflux-master/go.mod ./mainflux-master/go.sum ./
RUN go mod download
COPY . .

EXPOSE 1886

RUN cd mainflux-master/cmd/mqtt && go build main.go

CMD ["./main"]