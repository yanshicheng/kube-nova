package authutil

import (
	"testing"
)

func TestNormalizeBearerTokenStripsBearerPrefix(t *testing.T) {
	got := NormalizeBearerToken("kube-nova", "Bearer header.payload.signature")
	if got != "header.payload.signature" {
		t.Fatalf("unexpected token: %s", got)
	}
}

func TestNormalizeBearerTokenKeepsOtherChannelToken(t *testing.T) {
	raw := "opaque-token"
	got := NormalizeBearerToken("harbor", raw)
	if got != raw {
		t.Fatalf("unexpected token: %s", got)
	}
}
