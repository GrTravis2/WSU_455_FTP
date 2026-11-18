package client

import (
	"net"
	"testing"
	"time"
	"wsu455ftp/ftp/server"
)

func TestClientLogin(t *testing.T) {
	serverIp := "127.0.0.1"
	serverPort := "4567"
	// create a channel to store result of server run spawned in its own thread
	ctx := t.Context()
	runOutput := make(chan error, 1)
	go func() {
		runOutput <- server.Run(ctx, serverIp, serverPort)
	}()
	// Run this function when testing scope ends (its tied to t.Context lifetime)
	t.Cleanup(func() {
		select {
		case err := <-runOutput: // block on result of run function
			if err != nil {
				t.Errorf("server stopped with error: %v", err)
			}
		// start a timer channel, if it returns before
		case <-time.After(time.Second * 30):
			t.Error("server failed to stop after timeout")
		}
	})
	// loop until server is ready to accept connections before continuing
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(3 * time.Second)
	defer timeout.Stop()
	for {
		conn, err := net.DialTimeout("tcp", serverIp+":"+serverPort, 25*time.Millisecond)
		if err == nil { // once connection succeeds, break and go to real test
			conn.Close()
			break
		}
		select {
		case <-timeout.C:
			t.Fatalf("server failed to connect within timeout: %v", err)
		case <-ticker.C: // fall through to trigger retry every tick
		}
	}

	// create the client which completes login sequence
	client, err := NewSession(server.DEFAULT_USER, server.DEFAULT_PASS, serverIp, serverPort)
	if err != nil {
		t.Fatalf("error creating new control session: %v", err)
	}
	err = client.Quit()
	if err != nil {
		t.Fatalf("error quitting control session: %v", err)
	}
}
