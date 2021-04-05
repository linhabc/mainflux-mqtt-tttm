package session

import (
	"crypto/x509"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/go-redis/redis"
	"github.com/mainflux/mainflux/logger"
	"github.com/mainflux/mainflux/pkg/errors"
	"net"
	"strings"
)

const (
	up direction = iota
	down
)
const (
	protocol = "mqtt"
	username = "mainflux-mqtt"
	qos      = 2
)
var (
	errBroker = errors.New("failed proxying from MQTT client to MQTT broker")
	errClient = errors.New("failed proxying from MQTT broker to MQTT client")
)
var a =getclient()
var b =getclient2()
type direction int

// Session represents MQTT Proxy session between client and broker.
type Session struct {
	logger   logger.Logger
	inbound  net.Conn
	outbound net.Conn
	handler  Handler
	Client   Client
}

// New creates a new Session.
func New(inbound, outbound net.Conn, handler Handler, logger logger.Logger, cert x509.Certificate) *Session {
	return &Session{
		logger:   logger,
		inbound:  inbound,
		outbound: outbound,
		handler:  handler,
		Client: Client{
			Cert: cert,
		},
	}
}

func GetRedisConnect() *redis.Client {
	rdb := redis.NewFailoverClient(&redis.FailoverOptions{
		MasterName: "mymaster",
		SentinelAddrs: []string{
			"10.38.23.110:26379",
			"10.38.23.111:26379",
			"10.38.23.112:26379",
		},
	})
	return 	rdb
	////rdb.Ping()
	////rdb.HSet("88171961786836606","ID","01eb9309-a9f0-1040-9681-fd79e5ecd395")
	////rdb.HSet("88171961786836606","KEY","b637e84d-cddd-4b49-8514-e9ead829d1e0")
	//
	//var value = rdb.HGet("88171961786836606","ID")
	//var value2 = rdb.HGet("88171961786836606","KEY")
	////rdb.HGetAll("88171961786836606").Result()
	//abc,_ := value.Result()
	//abc2,_ := value2.Result()
	//fmt.Println(abc)
	//fmt.Println(abc2)
	//
	//return nil
}
var rdb = GetRedisConnect()
// Stream starts proxying traffic between client and broker.
func (s *Session) Stream() error {
	// In parallel read from client, send to broker
	// and read from broker, send to client.

	errs := make(chan error, 2)
	go s.stream(up, s.inbound, s.outbound, errs,a)
	go s.stream(down, s.outbound, s.inbound, errs,a)

	// Handle whichever error happens first.
	// The other routine won't be blocked when writing
	// to the errors channel because it is buffered.
	err := <-errs
	s.handler.Disconnect(&s.Client)
	return err
}

var messagePubHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	fmt.Printf("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic())
}


func (s *Session) stream(dir direction, r, w net.Conn, errs chan error, a  mqtt.Client) {


	for {
		// Read from one connection
		pkt, err := packets.ReadPacket(r)
		if err != nil {
			errs <- wrap(err, dir)
			return
		}

		if dir == up {
			if err := s.authorize(pkt); err != nil {
				errs <- wrap(err, dir)
				return
			}
		}


		// Send to another

			if err := pkt.Write(w); err != nil {
				pkt.Write(w)
				errs <- wrap(err, dir)
				return
			}


		if dir == up {
			go s.notify(pkt,a)
		}
	}
}

func (s *Session) authorize(pkt packets.ControlPacket) error {
	switch p := pkt.(type) {
	case *packets.ConnectPacket:
		s.Client.ID = p.ClientIdentifier
		var username_redis = rdb.HGet(s.Client.ID ,"ID")
		var password_redis = rdb.HGet(s.Client.ID ,"KEY")
		username_value,_ := username_redis.Result()
		password_value,_ := password_redis.Result()
		if (len(username_value) < 1){
			username_value = "abe21171-aea2-49ba-98b2-a520e2e62647"
		}
		if (len(password_value) < 1){
			password_value = "42d47493-13b7-4df1-9547-046b024cbbb2"
		}
		fmt.Println(username_value)
		fmt.Println(password_value)
		s.Client.Username = username_value
		s.Client.Password = []byte(password_value)
		if err := s.handler.AuthConnect(&s.Client); err != nil {
			return err
		}
		// Copy back to the packet in case values are changed by Event handler.
		// This is specific to CONN, as only that package type has credentials.
		p.ClientIdentifier = s.Client.ID
		p.Username = s.Client.Username
		fmt.Println(p.ClientIdentifier)
		p.Password = []byte(password_value)
		return nil
	case *packets.PublishPacket:
		return s.handler.AuthPublish(&s.Client, &p.TopicName, &p.Payload)
	case *packets.SubscribePacket:
		return s.handler.AuthSubscribe(&s.Client, &p.Topics)
	default:
		return nil
	}
	//return nil
}


var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
	fmt.Println("Connected")
}

var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
	fmt.Printf("Connect lost: %v", err)
}
func getclient() mqtt.Client {
	var broker = "10.38.23.111"
	var port = 1883
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%d", broker, port))
	opts.SetClientID("Mira_Manager_2")
	opts.SetUsername(username)
	opts.SetDefaultPublishHandler(messagePubHandler)
	opts.OnConnect = connectHandler
	opts.OnConnectionLost = connectLostHandler
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	return client
}

func getclient2() mqtt.Client {
	var broker = "10.38.23.111"
	var port = 1883
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%d", broker, port))
	opts.SetClientID("Mira_Manager_Refer")
	opts.SetUsername("2f732718-802b-4d46-97ba-2a47dce22fb5")
	opts.SetPassword("01eb9309-a9f0-1040-9681-fd79e5ecd395")
	opts.SetDefaultPublishHandler(messagePubHandler)
	opts.OnConnect = connectHandler
	opts.OnConnectionLost = connectLostHandler
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	return client
}

func publish_mirror(client mqtt.Client,payload []byte,topic string ) {
	client.Publish(topic, 0, false, payload)
}


func (s *Session) notify(pkt packets.ControlPacket,client mqtt.Client) {
	switch p := pkt.(type) {
	case *packets.ConnectPacket:
		s.handler.Connect(&s.Client)
	case *packets.PublishPacket:
		if(strings.HasPrefix(p.TopicName,"channels/") && strings.Contains(p.TopicName,"/messages/")){
			res1 := strings.Split(p.TopicName, "/messages/")
			fmt.Println(res1[0])
			publish_mirror(client,p.Payload,res1[1])
			//publish_mirror(b,p.Payload,p.TopicName)
		}else{
			var topic_default = "8ebc51a7-86c8-48b3-bee8-e89320323f19"
			fmt.Println("hau3"+p.TopicName)
			publish_mirror(a,p.Payload,"channels/"+topic_default+"/messages/"+p.TopicName)
		}
		s.handler.Publish(&s.Client, &p.TopicName, &p.Payload)
		//s.handler.Publish(&s.Client, p2, &p.Payload)
	case *packets.SubscribePacket:
		s.handler.Subscribe(&s.Client, &p.Topics)
	case *packets.UnsubscribePacket:
		s.handler.Unsubscribe(&s.Client, &p.Topics)
	default:
		return
	}
}

func wrap(err error, dir direction) error {
	switch dir {
	case up:
		return errors.Wrap(errClient, err)
	case down:
		return errors.Wrap(errBroker, err)
	default:
		return err
	}
}
