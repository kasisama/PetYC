package core

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestServerManagerKeepsBothPortsUntilConfirmed(t *testing.T) {
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.WriteString(writer, "shared-router")
	})
	var mu sync.Mutex
	persisted := []int{}
	manager := NewServerManager(handler, func(port int) error {
		mu.Lock()
		defer mu.Unlock()
		persisted = append(persisted, port)
		return nil
	})
	t.Cleanup(func() { _ = manager.Close() })
	if err := manager.Start(0); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	oldAddress := manager.Address()
	assertHTTPBody(t, oldAddress, "shared-router")

	handoff, err := manager.BeginPortHandoff(0)
	if err != nil {
		t.Fatalf("BeginPortHandoff() error = %v", err)
	}
	if handoff.ConfirmationToken == "" || handoff.Address == oldAddress {
		t.Fatalf("handoff = %#v", handoff)
	}
	assertHTTPBody(t, oldAddress, "shared-router")
	assertHTTPBody(t, handoff.Address, "shared-router")
	mu.Lock()
	if len(persisted) != 0 {
		t.Fatalf("port persisted before confirmation: %v", persisted)
	}
	mu.Unlock()

	if err := manager.ConfirmPortHandoff(handoff.ConfirmationToken); err != nil {
		t.Fatalf("ConfirmPortHandoff() error = %v", err)
	}
	mu.Lock()
	if len(persisted) != 1 || persisted[0] != handoff.Port {
		t.Fatalf("persisted ports = %v, want [%d]", persisted, handoff.Port)
	}
	mu.Unlock()
	assertHTTPBody(t, handoff.Address, "shared-router")
	assertEventuallyUnavailable(t, oldAddress)
	if err := manager.ConfirmPortHandoff(handoff.ConfirmationToken); err == nil {
		t.Fatal("one-time confirmation token was accepted twice")
	}
}

func TestServerManagerRollsBackUnconfirmedPortAfterTimeout(t *testing.T) {
	manager := NewServerManager(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}), func(int) error {
		t.Fatal("unconfirmed port must not be persisted")
		return nil
	})
	manager.handoffTimeout = 40 * time.Millisecond
	t.Cleanup(func() { _ = manager.Close() })
	if err := manager.Start(0); err != nil {
		t.Fatal(err)
	}
	oldAddress := manager.Address()
	handoff, err := manager.BeginPortHandoff(0)
	if err != nil {
		t.Fatal(err)
	}
	assertHTTPStatus(t, handoff.Address, http.StatusNoContent)
	assertEventuallyUnavailable(t, handoff.Address)
	assertHTTPStatus(t, oldAddress, http.StatusNoContent)
	if err := manager.ConfirmPortHandoff(handoff.ConfirmationToken); err == nil {
		t.Fatal("expired confirmation token was accepted")
	}
}

func TestServerManagerOccupiedPortDoesNotCreateHandoffOrPersist(t *testing.T) {
	occupied, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	occupiedPort := occupied.Addr().(*net.TCPAddr).Port

	manager := NewServerManager(http.NotFoundHandler(), func(int) error {
		t.Fatal("occupied port must not be persisted")
		return nil
	})
	t.Cleanup(func() { _ = manager.Close() })
	if err := manager.Start(0); err != nil {
		t.Fatal(err)
	}
	oldAddress := manager.Address()
	if _, err := manager.BeginPortHandoff(occupiedPort); err == nil {
		t.Fatal("BeginPortHandoff() accepted an occupied port")
	}
	if manager.Address() != oldAddress {
		t.Fatalf("active address changed: got %q, want %q", manager.Address(), oldAddress)
	}
}

func TestServerManagerEndpointHandoffPersistsListenAddress(t *testing.T) {
	var persistedHost string
	var persistedPort int
	manager := NewServerManagerWithEndpoint(http.NotFoundHandler(), func(host string, port int) error {
		persistedHost, persistedPort = host, port
		return nil
	})
	t.Cleanup(func() { _ = manager.Close() })
	if err := manager.StartEndpoint("127.0.0.1", 0); err != nil {
		t.Fatal(err)
	}
	handoff, err := manager.BeginEndpointHandoff("127.0.0.1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = manager.ConfirmPortHandoff(handoff.ConfirmationToken); err != nil {
		t.Fatal(err)
	}
	if persistedHost != "127.0.0.1" || persistedPort != handoff.Port {
		t.Fatalf("persisted endpoint = %s:%d, want 127.0.0.1:%d", persistedHost, persistedPort, handoff.Port)
	}
}

func assertHTTPBody(t *testing.T, address, want string) {
	t.Helper()
	response, err := http.Get(address)
	if err != nil {
		t.Fatalf("GET %s: %v", address, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != want {
		t.Fatalf("GET %s body = %q, want %q", address, body, want)
	}
}

func assertHTTPStatus(t *testing.T, address string, want int) {
	t.Helper()
	response, err := http.Get(address)
	if err != nil {
		t.Fatalf("GET %s: %v", address, err)
	}
	response.Body.Close()
	if response.StatusCode != want {
		t.Fatalf("GET %s status = %d, want %d", address, response.StatusCode, want)
	}
}

func assertEventuallyUnavailable(t *testing.T, address string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		client := &http.Client{Timeout: 50 * time.Millisecond}
		response, err := client.Get(address)
		if err != nil {
			return
		}
		response.Body.Close()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server %s remained available", fmt.Sprint(address))
}
