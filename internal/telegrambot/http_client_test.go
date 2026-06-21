package telegrambot

import "testing"

func TestNewTelegramHTTPClientEmpty(t *testing.T) {
	client, err := NewTelegramHTTPClient("")
	if err != nil {
		t.Fatalf("NewTelegramHTTPClient empty: %v", err)
	}
	if client != nil {
		t.Fatalf("client = %#v, want nil", client)
	}
}

func TestNewTelegramHTTPClientHTTPProxy(t *testing.T) {
	client, err := NewTelegramHTTPClient("http://proxy-host:3128")
	if err != nil {
		t.Fatalf("NewTelegramHTTPClient http: %v", err)
	}
	if client == nil || client.Transport == nil {
		t.Fatalf("client/transport not configured: %#v", client)
	}
}

func TestNewTelegramHTTPClientSOCKS5Proxy(t *testing.T) {
	client, err := NewTelegramHTTPClient("socks5://user:pass@127.0.0.1:1080")
	if err != nil {
		t.Fatalf("NewTelegramHTTPClient socks5: %v", err)
	}
	if client == nil || client.Transport == nil {
		t.Fatalf("client/transport not configured: %#v", client)
	}
}

func TestNewTelegramHTTPClientUnsupportedScheme(t *testing.T) {
	client, err := NewTelegramHTTPClient("ftp://proxy-host:21")
	if err == nil {
		t.Fatalf("expected unsupported scheme error, client=%#v", client)
	}
}
