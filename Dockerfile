  
FROM golang:alpine3.13

LABEL maintainer="linhnln"

WORKDIR /app
COPY ./mainflux-master/go.mod ./mainflux-master/go.sum ./
RUN go mod download
COPY . .

EXPOSE 1886 
EXPOSE 8080

RUN cd mainflux-master/cmd/mqtt && go build main.go 

CMD ["./mainflux-master/cmd/mqtt/main"]