package config

import "testing"

func TestValidateAllowsLoopbackWhenAuthModeNone(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:8080", "localhost:8080", "[::1]:8080"} {
		cfg := Config{
			HTTP: HTTPConfig{Addr: addr},
			Auth: AuthConfig{Mode: AuthModeNone},
		}

		if err := cfg.Validate(); err != nil {
			t.Fatalf("expected %s to be allowed: %v", addr, err)
		}
	}
}

func TestValidateRejectsPublicBindWhenAuthModeNone(t *testing.T) {
	for _, addr := range []string{"0.0.0.0:8080", "192.168.1.10:8080", ":8080"} {
		cfg := Config{
			HTTP: HTTPConfig{Addr: addr},
			Auth: AuthConfig{Mode: AuthModeNone},
		}

		if err := cfg.Validate(); err == nil {
			t.Fatalf("expected %s to be rejected", addr)
		}
	}
}

func TestValidateRequiresSessionSecretWhenAuthEnabled(t *testing.T) {
	cfg := Config{
		HTTP: HTTPConfig{Addr: "0.0.0.0:8080"},
		Auth: AuthConfig{Mode: "session"},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected empty SESSION_SECRET to be rejected when auth is enabled")
	}

	cfg.Session.Secret = "secret"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected session auth with secret to be allowed: %v", err)
	}
}
