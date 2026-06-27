package httpcheck

import (
	"net/http"
	"testing"
)

func TestVersionFromHeaderParsesNexusServer(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("Server", "Nexus/3.91.1-04 (COMMUNITY)")

	if got := versionFromHeader(resp); got != "3.91.1-04" {
		t.Fatalf("versionFromHeader = %q, want %q", got, "3.91.1-04")
	}
}

func TestVersionFromBodyPrefersJSONVersion(t *testing.T) {
	body := []byte(`{"Version":"v1.2.3","buildDate":"2026-06-04"}`)

	if got := versionFromBody(body, true); got != "v1.2.3" {
		t.Fatalf("versionFromBody = %q, want %q", got, "v1.2.3")
	}
}

func TestVersionFromBodySupportsPlainVersion(t *testing.T) {
	body := []byte("10.7.0.96327")

	if got := versionFromBody(body, true); got != "10.7.0.96327" {
		t.Fatalf("versionFromBody = %q, want %q", got, "10.7.0.96327")
	}
}
