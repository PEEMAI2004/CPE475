package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/potbuddy/local-node/internal/api"
	"github.com/potbuddy/local-node/internal/config"
	"github.com/potbuddy/local-node/internal/db"
	"github.com/potbuddy/local-node/internal/metrics"
	nodemqtt "github.com/potbuddy/local-node/internal/mqtt"
	"github.com/potbuddy/local-node/internal/processor"
	"github.com/potbuddy/local-node/internal/store"
)

func main() {
	// --- Load config ---
	cfgPath := "config.yaml"
	if v := os.Getenv("CONFIG_PATH"); v != "" {
		cfgPath = v
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	log.Printf("PotBuddy Local Node starting (device: %s)", cfg.DeviceID)

	// Set timezone offset from config
	processor.SetTimezoneOffset(cfg.TZOffset)
	log.Printf("[main] Timezone offset configured to UTC%+d", cfg.TZOffset)

	// Set Prometheus URL for historical sun data recovery
	if cfg.Prometheus.URL != "" {
		processor.SetPrometheusURL(cfg.Prometheus.URL)
		processor.SetPromLookbackHours(cfg.Prometheus.LookbackHours)
		log.Printf("[main] Prometheus recovery enabled via %s (lookback: %dh)", cfg.Prometheus.URL, cfg.Prometheus.LookbackHours)
	}

	// --- Database setup ---
	if cfg.Database.DSN != "" {
		if err := db.InitDB(cfg.Database.DSN); err != nil {
			log.Fatalf("failed to connect to database: %v", err)
		}
		// Start caching loop (fetches new profile updates every minute)
		go db.StartThresholdPoller(time.Minute)
	} else {
		log.Println("WARNING: running without database connection (hardcoded thresholds active)")
	}

	// --- Ring buffer ---
	rb := store.NewRingBuffer(cfg.Store.BufferSize)

	// --- MQTT publisher ---
	pub, err := nodemqtt.NewPublisher(cfg.LocalMQTT, cfg.CloudMQTT)
	if err != nil {
		log.Fatalf("failed to create publisher: %v", err)
	}
	defer pub.Disconnect()

	// --- Message channel ---
	msgCh := make(chan nodemqtt.Message, 64)

	// --- MQTT subscriber ---
	sub, err := nodemqtt.NewSubscriber(cfg.LocalMQTT, msgCh)
	if err != nil {
		log.Fatalf("failed to create subscriber: %v", err)
	}
	defer sub.Disconnect()

	// --- Online watchdog — marks devices offline after 30 s of silence ---
	metrics.StartOnlineWatchdog()

	// --- Processor goroutine ---
	go func() {
		for msg := range msgCh {
			// Extract device_id from topic: potbuddy/{device_id}/raw
			topicDeviceID := ""
			parts := strings.Split(msg.Topic, "/")
			if len(parts) >= 2 {
				topicDeviceID = parts[1]
			}

			if topicDeviceID == "" {
				log.Printf("[processor] warning: could not extract device_id from topic %q", msg.Topic)
				continue
			}

			// Validate payload identity against topic (mapped from cert CN by broker)
			if cfg.ValidateDeviceID {
				var payload struct {
					DeviceID string `json:"device_id"`
				}
				if err := json.Unmarshal(msg.Payload, &payload); err == nil {
					if payload.DeviceID != "" && payload.DeviceID != topicDeviceID {
						log.Printf("[processor] SECURITY ALERT: payload device_id %q does not match cert CN (topic) %q - DROPPING", payload.DeviceID, topicDeviceID)
						continue
					}
				}
			}

			// Auto-register new devices into database
			db.RegisterDevice(topicDeviceID)

			reading, err := processor.Parse(msg.Payload)
			if err != nil {
				log.Printf("[processor] parse error: %v", err)
				continue
			}
			enriched := processor.Enrich(reading, topicDeviceID)
			rb.Push(enriched)
			metrics.Update(enriched)

			log.Printf("[processor] device=%s status=%s  light=%.0f temp=%.0f hum=%.0f soil=%.0f",
				topicDeviceID,
				enriched.Status.Overall,
				derefOr(enriched.Raw.Light, -1),
				derefOr(enriched.Raw.Temp, -1),
				derefOr(enriched.Raw.Hum, -1),
				derefOr(enriched.Raw.Soil, -1),
			)

			if err := pub.PublishLocal(enriched); err != nil {
				log.Printf("[processor] local publish error: %v", err)
			}
			if err := pub.PublishCloud(enriched); err != nil {
				log.Printf("[processor] cloud publish error: %v", err)
			}
		}
	}()

	// --- HTTP API server ---
	httpHandler := api.NewHandler(rb)
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HTTP.Port),
		Handler:      httpHandler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Enable mTLS if configured
	if cfg.HTTP.CertFile != "" && cfg.HTTP.KeyFile != "" {
		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}

		if cfg.HTTP.CAFile != "" {
			caCert, err := os.ReadFile(cfg.HTTP.CAFile)
			if err != nil {
				log.Fatalf("failed to read HTTP CA file: %v", err)
			}
			caCertPool := x509.NewCertPool()
			if !caCertPool.AppendCertsFromPEM(caCert) {
				log.Fatalf("failed to parse HTTP CA certificate")
			}
			tlsConfig.ClientCAs = caCertPool
			tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		}

		srv.TLSConfig = tlsConfig

		go func() {
			log.Printf("[api] listening on https://0.0.0.0:%d (mTLS enabled)", cfg.HTTP.Port)
			if err := srv.ListenAndServeTLS(cfg.HTTP.CertFile, cfg.HTTP.KeyFile); err != nil && err != http.ErrServerClosed {
				log.Fatalf("[api] server error: %v", err)
			}
		}()
	} else {
		go func() {
			log.Printf("[api] listening on http://localhost:%d", cfg.HTTP.Port)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("[api] server error: %v", err)
			}
		}()
	}

	// --- Graceful shutdown ---
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}
	close(msgCh)
	log.Println("PotBuddy Local Node stopped.")
}

func derefOr(p *float64, fallback float64) float64 {
	if p == nil {
		return fallback
	}
	return *p
}
