// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package email

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// MockSMTPServer provides a simple mock SMTP server for testing and development
type MockSMTPServer struct {
	listener  net.Listener
	addr      string
	responses []string
}

// NewMockSMTPServer creates a new mock SMTP server
func NewMockSMTPServer(responses []string) (*MockSMTPServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	server := &MockSMTPServer{
		listener:  listener,
		addr:      listener.Addr().String(),
		responses: responses,
	}

	go server.serve()
	return server, nil
}

// NewMockSMTPServerForTesting creates a mock SMTP server for testing with require assertions
func NewMockSMTPServerForTesting(t *testing.T, responses []string) *MockSMTPServer {
	server, err := NewMockSMTPServer(responses)
	require.NoError(t, err)
	return server
}

// GetAddress returns the server address (host:port)
func (s *MockSMTPServer) GetAddress() string {
	return s.addr
}

// GetHost returns just the host part of the address
func (s *MockSMTPServer) GetHost() (string, error) {
	host, _, err := net.SplitHostPort(s.addr)
	return host, err
}

// GetPort returns just the port part of the address
func (s *MockSMTPServer) GetPort() (int, error) {
	_, portStr, err := net.SplitHostPort(s.addr)
	if err != nil {
		return 0, err
	}

	var port int
	_, err = fmt.Sscanf(portStr, "%d", &port)
	return port, err
}

// Close shuts down the mock server
func (s *MockSMTPServer) Close() error {
	return s.listener.Close()
}

func (s *MockSMTPServer) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return // Server closed
		}

		go s.handleConnection(conn)
	}
}

func (s *MockSMTPServer) handleConnection(conn net.Conn) {
	defer func() {
		_ = conn.Close() // Ignore close error in mock server
	}()

	reader := bufio.NewReader(conn)

	// Send initial greeting
	_, _ = conn.Write([]byte("220 localhost SMTP ready\r\n"))

	responseIndex := 0
	for {
		// Read client command
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}

		line = strings.TrimSpace(line)

		// Handle QUIT command specially
		if strings.HasPrefix(strings.ToUpper(line), "QUIT") {
			_, _ = conn.Write([]byte("221 Bye\r\n"))
			return
		}

		// Send appropriate response
		if responseIndex < len(s.responses) {
			_, _ = conn.Write([]byte(s.responses[responseIndex] + "\r\n"))
			responseIndex++
		} else {
			// Default response for any extra commands
			_, _ = conn.Write([]byte("250 OK\r\n"))
		}
	}
}

// DefaultSuccessfulSMTPResponses returns a set of responses for a successful SMTP session
func DefaultSuccessfulSMTPResponses() []string {
	return []string{
		"250 Hello",            // HELO/EHLO response
		"250 OK",               // MAIL FROM response
		"250 OK",               // RCPT TO response
		"354 Start mail input", // DATA response
		"250 OK",               // End of data response
	}
}

// DefaultFailureSMTPResponses returns a set of responses for a failed SMTP session
func DefaultFailureSMTPResponses() []string {
	return []string{
		"250 Hello",               // HELO/EHLO response
		"550 Mailbox unavailable", // MAIL FROM error
	}
}
