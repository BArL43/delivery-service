package utils

import (
	"testing"
	"time"
)

func TestTokenRoundTrip(t *testing.T) {
	manager, err := NewTokenManager("test-secret-that-is-longer-than-32-bytes", "delivery-auth", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	token, err := manager.Generate(42, "courier")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := manager.Parse(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != 42 || claims.Role != "courier" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestTokenManagerRejectsShortSecret(t *testing.T) {
	if _, err := NewTokenManager("short", "delivery-auth", time.Hour); err == nil {
		t.Fatal("expected short secret error")
	}
}

func TestTokenRejectsDifferentSecret(t *testing.T) {
	issuer := "delivery-auth"
	first, _ := NewTokenManager("first-secret-that-is-definitely-32-bytes-long", issuer, time.Hour)
	second, _ := NewTokenManager("second-secret-that-is-definitely-32-byte-long", issuer, time.Hour)
	token, err := first.Generate(7, "client")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := second.Parse(token); err == nil {
		t.Fatal("expected signature verification failure")
	}
}
