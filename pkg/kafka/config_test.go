package kafka

import (
	"crypto/tls"
	"testing"
)

func TestNewSaramaConfig_TLSDisabled(t *testing.T) {
	t.Parallel()

	cfg := NewSaramaConfig(false)
	if cfg.Net.TLS.Enable {
		t.Fatal("expected TLS to be disabled")
	}
	if cfg.Net.TLS.Config != nil {
		t.Fatal("expected nil TLS config when TLS is disabled")
	}
}

func TestNewSaramaConfig_TLSEnabled(t *testing.T) {
	t.Parallel()

	cfg := NewSaramaConfig(true)
	if !cfg.Net.TLS.Enable {
		t.Fatal("expected TLS to be enabled")
	}
	if cfg.Net.TLS.Config == nil {
		t.Fatal("expected TLS config")
	}
	if cfg.Net.TLS.Config.MinVersion != tls.VersionTLS12 {
		t.Fatalf("expected min TLS version %x, got %x", tls.VersionTLS12, cfg.Net.TLS.Config.MinVersion)
	}
}

func TestParseTLSEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		want    bool
		wantErr bool
	}{
		{name: "empty", value: "", want: false},
		{name: "true", value: "true", want: true},
		{name: "one", value: "1", want: true},
		{name: "false", value: "false", want: false},
		{name: "invalid", value: "yes", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseTLSEnabled(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}
