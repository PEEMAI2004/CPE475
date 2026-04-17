package mqtt

import (
	"encoding/json"
	"fmt"
	"log"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/potbuddy/local-node/internal/config"
	"github.com/potbuddy/local-node/internal/processor"
)

// Publisher publishes enriched payloads to one or more MQTT brokers.
type Publisher struct {
	localClient pahomqtt.Client
	cloudClient pahomqtt.Client
	localTopic  string
	cloudTopic  string
	cloudEnabled bool
}

func newClient(cfg config.MQTTConfig, suffix string) (pahomqtt.Client, error) {
	opts := pahomqtt.NewClientOptions().
		AddBroker(cfg.Broker).
		SetClientID(cfg.ClientID + "-" + suffix).
		SetCleanSession(true).
		SetAutoReconnect(true).
		SetOnConnectHandler(func(c pahomqtt.Client) {
			log.Printf("[publisher-%s] connected to %s", suffix, cfg.Broker)
		}).
		SetConnectionLostHandler(func(c pahomqtt.Client, err error) {
			log.Printf("[publisher-%s] connection lost: %v", suffix, err)
		})

	if cfg.Username != "" {
		opts.SetUsername(cfg.Username).SetPassword(cfg.Password)
	}

	client := pahomqtt.NewClient(opts)
	if tok := client.Connect(); tok.Wait() && tok.Error() != nil {
		return nil, fmt.Errorf("publisher-%s: connect to %s: %w", suffix, cfg.Broker, tok.Error())
	}
	return client, nil
}

// NewPublisher creates a Publisher that can publish to both local and cloud brokers.
// Cloud publishing is skipped (but not fatal) if the cloud broker is unreachable.
func NewPublisher(localCfg, cloudCfg config.MQTTConfig) (*Publisher, error) {
	localClient, err := newClient(localCfg, "local")
	if err != nil {
		return nil, err
	}

	p := &Publisher{
		localClient:  localClient,
		localTopic:   localCfg.PubTopic,
		cloudTopic:   cloudCfg.PubTopic,
		cloudEnabled: false,
	}

	// Cloud connection is best-effort — log a warning but keep running.
	cloudClient, err := newClient(cloudCfg, "cloud")
	if err != nil {
		log.Printf("[publisher] WARNING: cloud broker unavailable: %v — will retry on reconnect", err)
	} else {
		p.cloudClient = cloudClient
		p.cloudEnabled = true
	}

	return p, nil
}

func marshalPayload(payload processor.EnrichedPayload) ([]byte, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("publisher: marshal: %w", err)
	}
	return b, nil
}

// PublishLocal publishes the enriched payload to the local broker.
func (p *Publisher) PublishLocal(payload processor.EnrichedPayload) error {
	b, err := marshalPayload(payload)
	if err != nil {
		return err
	}
	tok := p.localClient.Publish(p.localTopic, 1, false, b)
	tok.Wait()
	if tok.Error() != nil {
		return fmt.Errorf("publisher: local publish: %w", tok.Error())
	}
	log.Printf("[publisher] local → %s: %s", p.localTopic, payload.Status.Overall)
	return nil
}

// PublishCloud publishes the enriched payload to the cloud broker.
// Returns nil silently if cloud is not enabled (graceful degradation).
func (p *Publisher) PublishCloud(payload processor.EnrichedPayload) error {
	if !p.cloudEnabled || p.cloudClient == nil {
		return nil
	}
	b, err := marshalPayload(payload)
	if err != nil {
		return err
	}
	tok := p.cloudClient.Publish(p.cloudTopic, 1, false, b)
	tok.Wait()
	if tok.Error() != nil {
		return fmt.Errorf("publisher: cloud publish: %w", tok.Error())
	}
	log.Printf("[publisher] cloud → %s: %s", p.cloudTopic, payload.Status.Overall)
	return nil
}

// Disconnect cleanly disconnects both MQTT clients.
func (p *Publisher) Disconnect() {
	p.localClient.Disconnect(500)
	if p.cloudClient != nil {
		p.cloudClient.Disconnect(500)
	}
}
