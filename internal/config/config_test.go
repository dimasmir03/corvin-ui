package config

import (
	"os"
	"testing"
)

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

func TestValidateAllowsDisabledTelegramWithoutToken(t *testing.T) {
	cfg := Config{
		HTTP:     HTTPConfig{Addr: "127.0.0.1:8080"},
		Auth:     AuthConfig{Mode: AuthModeNone},
		Telegram: TelegramConfig{Enabled: false},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected disabled telegram without token to be allowed: %v", err)
	}
}

func TestValidateRequiresTelegramTokenWhenEnabled(t *testing.T) {
	cfg := Config{
		HTTP:     HTTPConfig{Addr: "127.0.0.1:8080"},
		Auth:     AuthConfig{Mode: AuthModeNone},
		Telegram: TelegramConfig{Enabled: true},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected enabled telegram without token to be rejected")
	}

	cfg.Telegram.Token = "token"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected enabled telegram with token to be allowed: %v", err)
	}
}

func TestLoadDefaultsRabbitMQTopology(t *testing.T) {
	keys := []string{"AMQP_EXCHANGE_COMMANDS", "RABBITMQ_EVENTS_EXCHANGE", "RABBITMQ_EVENTS_QUEUE", "RABBITMQ_EVENTS_ROUTING_KEY"}
	originals := map[string]*string{}
	for _, key := range keys {
		if value, ok := os.LookupEnv(key); ok {
			copyValue := value
			originals[key] = &copyValue
		} else {
			originals[key] = nil
		}
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset %s: %v", key, err)
		}
	}
	defer func() {
		for _, key := range keys {
			if originals[key] == nil {
				_ = os.Unsetenv(key)
				continue
			}
			_ = os.Setenv(key, *originals[key])
		}
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Defaults.AMQPExchangeCommands != "corvin.job.commands" {
		t.Fatalf("commands exchange = %q", cfg.Defaults.AMQPExchangeCommands)
	}
	if cfg.RabbitMQ.EventsExchange != "corvin.agent.events" || cfg.RabbitMQ.EventsQueue != "corvin.agent.events.panel" || cfg.RabbitMQ.EventsRouting != "node.snapshot" {
		t.Fatalf("unexpected events topology: %#v", cfg.RabbitMQ)
	}
}
