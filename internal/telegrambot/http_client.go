package telegrambot

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

const telegramHTTPClientTimeout = 30 * time.Second

func NewTelegramHTTPClient(proxyURL string) (*http.Client, error) {
	proxyURL = strings.TrimSpace(proxyURL)
	if proxyURL == "" {
		return nil, nil
	}

	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("invalid telegram proxy url: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid telegram proxy url")
	}

	transport := &http.Transport{}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		transport.Proxy = http.ProxyURL(parsed)
	case "socks5", "socks5h":
		dialer, err := proxy.FromURL(parsed, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("invalid telegram socks proxy: %w", err)
		}
		contextDialer, ok := dialer.(proxy.ContextDialer)
		if ok {
			transport.DialContext = contextDialer.DialContext
		} else {
			transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				type result struct {
					conn net.Conn
					err  error
				}
				ch := make(chan result, 1)
				go func() {
					conn, err := dialer.Dial(network, addr)
					ch <- result{conn: conn, err: err}
				}()
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case res := <-ch:
					return res.conn, res.err
				}
			}
		}
	default:
		return nil, fmt.Errorf("unsupported telegram proxy scheme %q", parsed.Scheme)
	}

	return &http.Client{Transport: transport, Timeout: telegramHTTPClientTimeout}, nil
}

func telegramProxyLogFields(proxyURL string) []any {
	parsed, err := url.Parse(strings.TrimSpace(proxyURL))
	if err != nil {
		return nil
	}
	return []any{"scheme", parsed.Scheme, "host", parsed.Hostname()}
}
