package client

import (
	"bufio"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
	"wsu455ftp/ftp/common"
	"wsu455ftp/ftp/constants"
	pi "wsu455ftp/ftp/protocolInterpreter"
	"wsu455ftp/ftp/server"
)

func newTestClient(t *testing.T) (*clientSession, net.Conn) {
	t.Helper()
	conn, peer := net.Pipe()
	t.Cleanup(func() {
		conn.Close()
		peer.Close()
	})
	conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	peer.SetDeadline(time.Now().Add(3 * time.Second))
	return &clientSession{openConn: conn, reader: *bufio.NewReader(conn)}, peer
}

func TestClientSettings(t *testing.T) {
	var testCases = []struct {
		caseName string
		command  string
		send     func(*clientSession) error
	}{
		{"TYPE ASCII", "TYPE A N\r\n", func(client *clientSession) error { return client.SetType(pi.ASCII, pi.NONPRINT, 0) }},
		{"TYPE Image", "TYPE I\r\n", func(client *clientSession) error { return client.SetType(pi.IMAGE, pi.NONE, 0) }},
		{"TYPE Telnet", "TYPE A T\r\n", func(client *clientSession) error { return client.SetType(pi.ASCII, pi.TELNET, 0) }},
		{"TYPE EBCDIC", "TYPE E N\r\n", func(client *clientSession) error { return client.SetType(pi.EBCDIC, pi.NONPRINT, 0) }},
		{"TYPE Local", "TYPE L 8\r\n", func(client *clientSession) error { return client.SetType(pi.LOCAL, pi.NONE, 8) }},
		{"MODE Stream", "MODE S\r\n", func(client *clientSession) error { return client.SetMode(pi.STREAM) }},
		{"MODE Block", "MODE B\r\n", func(client *clientSession) error { return client.SetMode(pi.BLOCK) }},
		{"MODE Compressed", "MODE C\r\n", func(client *clientSession) error { return client.SetMode(pi.COMPRESSED) }},
		{"STRU File", "STRU F\r\n", func(client *clientSession) error { return client.SetStru(pi.FILE) }},
		{"STRU Record", "STRU R\r\n", func(client *clientSession) error { return client.SetStru(pi.RECORD) }},
		{"STRU Page", "STRU P\r\n", func(client *clientSession) error { return client.SetStru(pi.PAGE) }},
	}
	var replies = []struct {
		caseName string
		reply    pi.FTPReply
		wantErr  bool
		rejected bool
	}{
		{"success", constants.CommandOkReply, false, false},
		{"different success message", pi.MakeFTPReply(200, "Setting accepted"), false, false},
		{"invalid arguments", pi.MakeFTPReply(501, "Syntax error in parameters or arguments"), true, true},
		{"unsupported parameter", pi.MakeFTPReply(504, "Command not implemented for that parameter"), true, true},
		{"unexpected reply", pi.MakeFTPReply(202, "Command not implemented, superfluous at this site"), true, false},
	}
	for _, tc := range testCases {
		for _, response := range replies {
			t.Run(tc.caseName+"/"+response.caseName, func(t *testing.T) {
				client, peer := newTestClient(t)
				done := make(chan struct{})
				go func() {
					defer close(done)
					defer peer.Close()
					reader := bufio.NewReader(peer)
					for i, command := range []string{tc.command, "QUIT\r\n"} {
						actual, err := reader.ReadString('\n')
						if err != nil {
							t.Errorf("unable to read command: %v", err)
							return
						}
						if actual != command {
							t.Errorf("found command %q, expected %q", actual, command)
						}
						reply := response.reply
						if i == 1 {
							reply = constants.QuitConnectionReply
						}
						if err := common.SendReply(peer, reply); err != nil {
							t.Errorf("unable to send reply: %v", err)
							return
						}
					}
				}()
				err := tc.send(client)
				if (err != nil) != response.wantErr {
					t.Errorf("found error %v, expected error: %v", err, response.wantErr)
				}
				if errors.Is(err, ErrCommandRejected) != response.rejected {
					t.Errorf("found error %v, expected command rejection: %v", err, response.rejected)
				}
				if response.wantErr && err != nil && !strings.Contains(err.Error(), response.reply.String()) {
					t.Errorf("error does not include server reply: %v", err)
				}
				if err := client.Quit(); err != nil {
					t.Errorf("unable to quit after setting command: %v", err)
				}
				<-done
			})
		}
	}
}

func TestClientSettingsIOErrors(t *testing.T) {
	var testCases = []struct {
		caseName string
		send     func(*clientSession) error
	}{
		{"TYPE", func(client *clientSession) error { return client.SetType(pi.IMAGE, pi.NONE, 0) }},
		{"MODE", func(client *clientSession) error { return client.SetMode(pi.STREAM) }},
		{"STRU", func(client *clientSession) error { return client.SetStru(pi.FILE) }},
	}
	for _, tc := range testCases {
		for _, failure := range []string{"write error", "read error", "malformed reply"} {
			t.Run(tc.caseName+"/"+failure, func(t *testing.T) {
				client, peer := newTestClient(t)
				done := make(chan struct{})
				go func() {
					defer close(done)
					defer peer.Close()
					if failure == "write error" {
						return
					}
					if _, err := bufio.NewReader(peer).ReadString('\n'); err != nil {
						t.Errorf("unable to read command: %v", err)
						return
					}
					if failure == "malformed reply" {
						if _, err := io.WriteString(peer, "invalid reply\r\n"); err != nil {
							t.Errorf("unable to send malformed reply: %v", err)
						}
					}
				}()
				if err := tc.send(client); err == nil || errors.Is(err, ErrCommandRejected) {
					t.Errorf("expected connection or reply error, found %v", err)
				}
				<-done
			})
		}
	}
}

func TestClientClose(t *testing.T) {
	for _, caseName := range []string{"idle", "waiting for reply"} {
		t.Run(caseName, func(t *testing.T) {
			client, peer := newTestClient(t)
			result := make(chan error, 1)
			if caseName == "waiting for reply" {
				go func() { result <- client.SetMode(pi.STREAM) }()
				if _, err := bufio.NewReader(peer).ReadString('\n'); err != nil {
					t.Fatal(err)
				}
			}
			if err := client.Close(); err != nil {
				t.Fatalf("unable to close client connection: %v", err)
			}
			if caseName == "waiting for reply" {
				select {
				case err := <-result:
					if err == nil || errors.Is(err, ErrCommandRejected) {
						t.Errorf("expected connection error after close, found %v", err)
					}
				case <-time.After(3 * time.Second):
					t.Fatal("closing client did not unblock command")
				}
			}
			if b, err := bufio.NewReader(peer).ReadByte(); err != io.EOF {
				t.Fatalf("expected EOF without a QUIT command, found byte %q and error %v", b, err)
			}
		})
	}
}

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
	client, err := NewSession(constants.DEFAULT_USER, constants.DEFAULT_PASS, serverIp, serverPort)
	if err != nil {
		t.Fatalf("error creating new control session: %v", err)
	}
	err = client.Quit()
	if err != nil {
		t.Fatalf("error quitting control session: %v", err)
	}
}
