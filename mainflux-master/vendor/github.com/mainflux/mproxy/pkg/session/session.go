package session

import (
	"crypto/x509"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/eclipse/paho.mqtt.golang/packets"
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
		//fmt.Println(pkt.String())


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
	//switch p := pkt.(type) {
	//case *packets.ConnectPacket:
	//	s.Client.ID = p.ClientIdentifier
	//	s.Client.Username = "ac79f69b-977d-46c4-b42d-b23492ed2dab"
	//	s.Client.Password = []byte("2e4c6c7a-aedc-4a67-9f35-11e73534fe20")
	//	if err := s.handler.AuthConnect(&s.Client); err != nil {
	//		return err
	//	}
	//	// Copy back to the packet in case values are changed by Event handler.
	//	// This is specific to CONN, as only that package type has credentials.
	//	p.ClientIdentifier = s.Client.ID
	//	p.Username = s.Client.Username
	//	p.Password = []byte("2e4c6c7a-aedc-4a67-9f35-11e73534fe20")
	//	return nil
	//case *packets.PublishPacket:
	//	return s.handler.AuthPublish(&s.Client, &p.TopicName, &p.Payload)
	//case *packets.SubscribePacket:
	//	return s.handler.AuthSubscribe(&s.Client, &p.Topics)
	//default:
	//	return nil
	//}
	return nil
}


var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
	fmt.Println("Connected")
}

var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
	fmt.Printf("Connect lost: %v", err)
}
func getclient() mqtt.Client {
	var broker = "10.38.23.111"
	var port = 31885
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%d", broker, port))
	opts.SetClientID("Mira_Manager")
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
func publish_mirror(client mqtt.Client,payload []byte,topic string ) {
	client.Publish(topic, 0, false, payload)
}


func (s *Session) notify(pkt packets.ControlPacket,client mqtt.Client) {
	switch p := pkt.(type) {
	case *packets.ConnectPacket:
		s.handler.Connect(&s.Client)
	case *packets.PublishPacket:
		if(strings.HasPrefix(p.TopicName,"channels/21a08024-aaeb-4bf9-9f42-5d24f74cfbda/messages/")){
			res1 := strings.Split(p.TopicName, "channels/21a08024-aaeb-4bf9-9f42-5d24f74cfbda/messages/")
			publish_mirror(client,p.Payload,res1[1])
		}else{
			publish_mirror(client,p.Payload,"channels/21a08024-aaeb-4bf9-9f42-5d24f74cfbda/messages/"+p.TopicName)
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
