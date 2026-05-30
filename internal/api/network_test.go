package api

import (
	"net"
	"net/http"
	"testing"
)

func Test_isLocalhostHost(t *testing.T) {
	t.Parallel()

	if !isLocalhostHost("localhost:8080") {
		t.Fatal("localhost:8080 should be local")
	}
	if !isLocalhostHost("127.0.0.1") {
		t.Fatal("127.0.0.1 should be local")
	}
	if isLocalhostHost("192.168.1.20:8080") {
		t.Fatal("192.168.1.20:8080 should not be local")
	}
}

func Test_lanAddressScorePrefersPhysicalPrivateAddresses(t *testing.T) {
	t.Parallel()

	physical := lanAddressScore("Wi-Fi", net.IPv4(192, 168, 1, 20))
	virtual := lanAddressScore("Docker Desktop", net.IPv4(172, 18, 0, 2))
	if physical <= virtual {
		t.Fatalf("physical score = %d, virtual score = %d, want physical higher", physical, virtual)
	}
}

func Test_requestSchemeAndHostPort(t *testing.T) {
	t.Parallel()

	request := &http.Request{Host: "localhost:8080", Header: http.Header{"X-Forwarded-Proto": []string{"https"}}}
	if got := requestScheme(request); got != "https" {
		t.Fatalf("scheme = %q, want https", got)
	}
	if got := hostPort("example.com", "https"); got != "443" {
		t.Fatalf("https default port = %q, want 443", got)
	}
	if got := hostPort("example.com:8080", "http"); got != "8080" {
		t.Fatalf("explicit port = %q, want 8080", got)
	}
}
