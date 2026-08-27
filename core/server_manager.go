package core

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

const defaultPortHandoffTimeout = 60 * time.Second

type PortHandoff struct {
	Port              int
	Address           string
	ConfirmationToken string
	ExpiresAt         time.Time
}

type managedHTTPServer struct {
	port     int
	address  string
	server   *http.Server
	listener net.Listener
	bindHost string
	timer    *time.Timer
	token    string
}

type ServerManager struct {
	mu              sync.Mutex
	handler         http.Handler
	persistEndpoint func(string, int) error
	handoffTimeout  time.Duration
	active          *managedHTTPServer
	pending         *managedHTTPServer
	errors          chan error
}

func NewServerManager(handler http.Handler, persistPort func(int) error) *ServerManager {
	return NewServerManagerWithEndpoint(handler, func(_ string, port int) error {
		if persistPort == nil {
			return nil
		}
		return persistPort(port)
	})
}

func NewServerManagerWithEndpoint(handler http.Handler, persistEndpoint func(string, int) error) *ServerManager {
	return &ServerManager{
		handler: handler, persistEndpoint: persistEndpoint, handoffTimeout: defaultPortHandoffTimeout,
		errors: make(chan error, 1),
	}
}

func (manager *ServerManager) Start(port int) error {
	return manager.StartEndpoint("127.0.0.1", port)
}

func (manager *ServerManager) StartEndpoint(host string, port int) error {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.active != nil {
		return errors.New("server is already running")
	}
	server, err := manager.listen(host, port)
	if err != nil {
		return err
	}
	manager.active = server
	return nil
}

func (manager *ServerManager) Address() string {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.active == nil {
		return ""
	}
	return manager.active.address
}

func (manager *ServerManager) BeginPortHandoff(port int) (PortHandoff, error) {
	manager.mu.Lock()
	host := "127.0.0.1"
	if manager.active != nil {
		host = manager.active.bindHost
	}
	manager.mu.Unlock()
	return manager.BeginEndpointHandoff(host, port)
}

func (manager *ServerManager) BeginEndpointHandoff(host string, port int) (PortHandoff, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.active == nil {
		return PortHandoff{}, errors.New("server is not running")
	}
	if manager.pending != nil {
		return PortHandoff{}, errors.New("another port handoff is pending")
	}
	server, err := manager.listen(host, port)
	if err != nil {
		return PortHandoff{}, err
	}
	token, err := newConfirmationToken()
	if err != nil {
		_ = server.listener.Close()
		return PortHandoff{}, err
	}
	expiresAt := time.Now().Add(manager.handoffTimeout)
	server.token = token
	manager.pending = server
	server.timer = time.AfterFunc(manager.handoffTimeout, func() {
		manager.rollbackPortHandoff(server)
	})
	return PortHandoff{Port: server.port, Address: server.address, ConfirmationToken: token, ExpiresAt: expiresAt}, nil
}

func (manager *ServerManager) ConfirmPortHandoff(token string) error {
	manager.mu.Lock()
	pending := manager.pending
	if pending == nil || token == "" || token != pending.token {
		manager.mu.Unlock()
		return errors.New("port confirmation token is invalid or expired")
	}
	if manager.persistEndpoint != nil {
		if err := manager.persistEndpoint(pending.bindHost, pending.port); err != nil {
			manager.mu.Unlock()
			return fmt.Errorf("persist confirmed port: %w", err)
		}
	}
	if pending.timer != nil {
		pending.timer.Stop()
	}
	old := manager.active
	pending.token = ""
	pending.timer = nil
	manager.active = pending
	manager.pending = nil
	manager.mu.Unlock()

	go shutdownManagedServer(old)
	return nil
}

func (manager *ServerManager) Wait() error {
	return <-manager.errors
}

func (manager *ServerManager) WaitContext(ctx context.Context) error {
	select {
	case err := <-manager.errors:
		return err
	case <-ctx.Done():
		return nil
	}
}

func (manager *ServerManager) Close() error {
	manager.mu.Lock()
	active := manager.active
	pending := manager.pending
	manager.active = nil
	manager.pending = nil
	if pending != nil && pending.timer != nil {
		pending.timer.Stop()
	}
	manager.mu.Unlock()

	var closeError error
	if pending != nil {
		closeError = shutdownManagedServer(pending)
	}
	if active != nil {
		if err := shutdownManagedServer(active); err != nil && closeError == nil {
			closeError = err
		}
	}
	return closeError
}

func (manager *ServerManager) listen(host string, port int) (*managedHTTPServer, error) {
	if host == "" {
		host = "127.0.0.1"
	}
	listener, err := net.Listen("tcp", net.JoinHostPort(host, fmt.Sprint(port)))
	if err != nil {
		return nil, fmt.Errorf("listen on %s:%d: %w", host, port, err)
	}
	actualPort := listener.Addr().(*net.TCPAddr).Port
	clientHost := host
	if clientHost == "0.0.0.0" || clientHost == "::" {
		clientHost = "127.0.0.1"
	}
	server := &http.Server{Handler: manager.handler}
	managed := &managedHTTPServer{
		port: actualPort, address: "http://" + net.JoinHostPort(clientHost, fmt.Sprint(actualPort)), bindHost: host, server: server, listener: listener,
	}
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			select {
			case manager.errors <- err:
			default:
			}
		}
	}()
	return managed, nil
}

func (manager *ServerManager) rollbackPortHandoff(server *managedHTTPServer) {
	manager.mu.Lock()
	if manager.pending != server {
		manager.mu.Unlock()
		return
	}
	manager.pending = nil
	server.token = ""
	server.timer = nil
	manager.mu.Unlock()
	_ = shutdownManagedServer(server)
}

func shutdownManagedServer(server *managedHTTPServer) error {
	if server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return server.server.Shutdown(ctx)
}

func newConfirmationToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate port confirmation token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
