package core

import "testing"

func TestValidWebSocketToken(t *testing.T) {
	if !validWebSocketToken("expected-token", "expected-token") {
		t.Fatal("validWebSocketToken rejected the expected token")
	}
	if validWebSocketToken("wrong-token", "expected-token") {
		t.Fatal("validWebSocketToken accepted a wrong token")
	}
}
