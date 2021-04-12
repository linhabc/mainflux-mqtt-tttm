  
FROM golang:alpine3.13

LABEL maintainer="linhnln"

# /mainflux-master/cmd/matt/main.go
ENV DEF_MQTT_PORT = 1886
ENV DEF_MQTT_TARGET_HOST = "10.38.23.111" 
ENV DEF_NATS_URL = "nats://10.38.23.111:31422"

# /mainflux-mqtt-tttm/mainflux-master/vendor/github.com/mainflux/mproxy/pkg/session/session.go
ENV USERNAME_VALUE = "abe21171-aea2-49ba-98b2-a520e2e62647"
ENV PASSWORD_VALUE = "42d47493-13b7-4df1-9547-046b024cbbb2"

ENV SENTINEL_ADDR_1 = "10.38.23.110:26379"
ENV SENTINEL_ADDR_2 = "10.38.23.111:26379"
ENV SENTINEL_ADDR_3 = "10.38.23.112:26379"

ENV BROKER_IP = "10.38.23.111"
ENV BROKER_PORT = 1883

ENV CLIENT_USERNAME = "2f732718-802b-4d46-97ba-2a47dce22fb5"
ENV CLIENT_PASSWORD = "01eb9309-a9f0-1040-9681-fd79e5ecd395"

ENV TOPIC_DEFAULT = "8ebc51a7-86c8-48b3-bee8-e89320323f19"

WORKDIR /app
COPY ./mainflux-master/go.mod ./mainflux-master/go.sum ./
RUN go mod download
COPY . .

EXPOSE 1886 
EXPOSE 8080

RUN cd mainflux-master/cmd/mqtt && go build main.go 

CMD ["./mainflux-master/cmd/mqtt/main"]