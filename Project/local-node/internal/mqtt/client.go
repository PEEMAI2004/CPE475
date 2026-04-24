package mqtt

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/potbuddy/local-node/internal/config"
)

func newTLSConfig(cfg config.MQTTConfig) (*tls.Config, error) {
	if cfg.CAFile == "" && cfg.CertFile == "" && cfg.KeyFile == "" {
		return nil, nil
	}

	tlsConfig := &tls.Config{}

	if cfg.CAFile != "" {
		caCert, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read ca file: %w", err)
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		tlsConfig.RootCAs = caCertPool
	}

	if cfg.CertFile != "" && cfg.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load key pair: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}

func createClient(cfg config.MQTTConfig, suffix string) (pahomqtt.Client, error) {
	opts := pahomqtt.NewClientOptions().
		AddBroker(cfg.Broker).
		SetClientID(cfg.ClientID + "-" + suffix).
		SetCleanSession(true).
		SetAutoReconnect(true).
		SetOnConnectHandler(func(c pahomqtt.Client) {
			log.Printf("[%s] connected to %s", suffix, cfg.Broker)
		}).
		SetConnectionLostHandler(func(c pahomqtt.Client, err error) {
			log.Printf("[%s] connection lost: %v", suffix, err)
		})

	if cfg.Username != "" {
		opts.SetUsername(cfg.Username).SetPassword(cfg.Password)
	}

	tlsConfig, err := newTLSConfig(cfg)
	if err != nil {
		return nil, err
	}
	if tlsConfig != nil {
		opts.SetTLSConfig(tlsConfig)
	}

	client := pahomqtt.NewClient(opts)
	if tok := client.Connect(); tok.Wait() && tok.Error() != nil {
		return nil, fmt.Errorf("%s: connect to %s: %w", suffix, cfg.Broker, tok.Error())
	}
	return client, nil
}
