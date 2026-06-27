package kafka

import (
	"crypto/tls"
	"fmt"
	"strconv"
	"strings"

	"github.com/IBM/sarama"
)

// NewSaramaConfig returns a baseline Sarama config shared by producers and
// consumers. MSK's TLS listener requires TLS to be enabled explicitly.
func NewSaramaConfig(tlsEnabled bool) *sarama.Config {
	cfg := sarama.NewConfig()
	if tlsEnabled {
		cfg.Net.TLS.Enable = true
		cfg.Net.TLS.Config = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	return cfg
}

// ParseTLSEnabled parses KAFKA_TLS-style environment values.
func ParseTLSEnabled(value string) (bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return false, nil
	}
	enabled, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("parse KAFKA_TLS: %w", err)
	}
	return enabled, nil
}
