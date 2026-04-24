package mqtt

import (
	"fmt"
	"log"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/potbuddy/local-node/internal/config"
)

// Message carries a raw MQTT payload together with the source topic.
// The topic encodes the device ID: potbuddy/{device_id}/raw
type Message struct {
	Topic   string
	Payload []byte
}

// Subscriber connects to the local broker and delivers raw messages to a channel.
type Subscriber struct {
	client pahomqtt.Client
	topic  string
}

// NewSubscriber creates a Subscriber connected to the local MQTT broker.
// Messages from sub_topic are sent to msgCh.
func NewSubscriber(cfg config.MQTTConfig, msgCh chan<- Message) (*Subscriber, error) {
	client, err := createClient(cfg, "subscriber")
	if err != nil {
		return nil, err
	}

	s := &Subscriber{client: client, topic: cfg.SubTopic}

	// Subscribe immediately after connecting.
	tok := client.Subscribe(cfg.SubTopic, 1, func(_ pahomqtt.Client, msg pahomqtt.Message) {
		// Copy payload — the library may reuse the underlying buffer.
		payload := make([]byte, len(msg.Payload()))
		copy(payload, msg.Payload())
		msgCh <- Message{Topic: msg.Topic(), Payload: payload}
	})
	if tok.Wait() && tok.Error() != nil {
		return nil, fmt.Errorf("subscriber: subscribe %q: %w", cfg.SubTopic, tok.Error())
	}

	log.Printf("[subscriber] subscribed to %s", cfg.SubTopic)
	return s, nil
}

// Disconnect cleanly disconnects the MQTT client.
func (s *Subscriber) Disconnect() {
	s.client.Disconnect(500)
}
