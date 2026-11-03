// WSU 455 Semester Project - FTP
// FTP Server Tests
package server

import (
	"bufio"
	"io"
	"net"
	"sync"
	"testing"
	"time"
	protocolinterpreter "wsu455ftp/ftp/protocolInterpreter"
	nvtascii "wsu455ftp/telnet/nvt-ascii"
)

func TestFTPReplyToString(t *testing.T) {
	var testCases = []struct {
		input    FTPReply
		expected string
		caseName string
	}{
		{FTPReply{[3]byte{2, 2, 0}, "Service ready"}, "220 Service ready\r\n", "220 greeting"},
		{FTPReply{[3]byte{2, 0, 0}, "Command okay"}, "200 Command okay\r\n", "200 command success"},
		{FTPReply{[3]byte{2, 2, 1}, "Service closing control connection"}, "221 Service closing control connection\r\n", "221 connection closing"},
		{FTPReply{[3]byte{5, 0, 0}, "Syntax error, command unrecognized"}, "500 Syntax error, command unrecognized\r\n", "500 error"},
	}

	for _, tc := range testCases {
		actual := tc.input.ToString()
		if actual != tc.expected {
			t.Fatalf("%s case: found %q, expected %q", tc.caseName, actual, tc.expected)
		}
		b, err := nvtascii.FromString(actual)
		if err != nil {
			t.Fatalf("%s case: %v", tc.caseName, err.Error())
		}
		if string(b) != tc.expected {
			t.Fatalf("%s case: found NVT ASCII %q, expected %q", tc.caseName, b, tc.expected)
		}
	}
}

// start a control session and check cleanup when it ends
func startControlSession(t *testing.T, conn net.Conn, session controlSession) func() {
	t.Helper()
	active_sessions := map[net.Conn]struct{}{conn: {}}
	var mutex sync.Mutex
	var wg sync.WaitGroup
	wg.Add(1)
	go controlSessionHandler(conn, session, &wg, &mutex, active_sessions)
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	wait := func() {
		t.Helper()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("control session did not finish")
		}
		mutex.Lock()
		defer mutex.Unlock()
		if len(active_sessions) != 0 {
			t.Fatal("terminated session is still active")
		}
	}
	t.Cleanup(func() {
		conn.Close()
		wait()
	})
	return wait
}

func readControlReply(t *testing.T, reader *bufio.Reader, expected string) {
	t.Helper()
	actual, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("unable to read reply: %v", err)
	}
	if actual != expected {
		t.Fatalf("found reply %q, expected %q", actual, expected)
	}
}

func TestControlSessionLifecycle(t *testing.T) {
	conn, client := net.Pipe()
	t.Cleanup(func() { client.Close() })
	client.SetDeadline(time.Now().Add(3 * time.Second))
	wait := startControlSession(t, conn, controlSession{})
	reader := bufio.NewReader(client)
	readControlReply(t, reader, "220 Service ready\r\n")

	// repeated commands show that NOOP leaves the connection open
	for i := 0; i < 2; i++ {
		if _, err := io.WriteString(client, "NOOP\r\n"); err != nil {
			t.Fatal(err)
		}
		readControlReply(t, reader, "200 Command okay\r\n")
	}
	// a parsed but unimplemented command must not produce 000
	if _, err := io.WriteString(client, "USER alice\r\n"); err != nil {
		t.Fatal(err)
	}
	readControlReply(t, reader, "502 Command not implemented\r\n")

	// buffered commands after QUIT must not be processed
	if _, err := io.WriteString(client, "QUIT\r\nNOOP\r\n"); err != nil {
		t.Fatal(err)
	}
	readControlReply(t, reader, "221 Service closing control connection\r\n")
	if b, err := reader.ReadByte(); err != io.EOF {
		t.Fatalf("expected EOF after QUIT, found byte %q and error %v", b, err)
	}
	wait()
}

func TestControlSessionDisconnect(t *testing.T) {
	for _, caseName := range []string{"before greeting", "EOF", "partial command", "read error"} {
		t.Run(caseName, func(t *testing.T) {
			conn, client := net.Pipe()
			t.Cleanup(func() { client.Close() })
			client.SetDeadline(time.Now().Add(3 * time.Second))
			if caseName == "before greeting" {
				client.Close()
			}
			wait := startControlSession(t, conn, controlSession{})
			if caseName != "before greeting" {
				readControlReply(t, bufio.NewReader(client), "220 Service ready\r\n")
				if caseName == "partial command" {
					if _, err := io.WriteString(client, "NOOP\r"); err != nil {
						t.Fatal(err)
					}
				}
				if caseName == "read error" {
					conn.Close()
				} else {
					client.Close()
				}
			}
			wait()
		})
	}
}

// fail a selected write while keeping the other end of the pipe open
type failingWriteConn struct {
	net.Conn
	writes int
	failAt int
	short  bool
}

func (conn *failingWriteConn) Write(b []byte) (int, error) {
	conn.writes++
	if conn.writes == conn.failAt {
		if conn.short {
			return len(b) - 1, nil
		}
		return 0, io.ErrClosedPipe
	}
	return conn.Conn.Write(b)
}

func TestControlSessionFailedWrite(t *testing.T) {
	var testCases = []struct {
		caseName string
		command  string
		failAt   int
		short    bool
	}{
		{"greeting", "", 1, false},
		{"NOOP reply", "NOOP\r\n", 2, false},
		{"QUIT reply", "QUIT\r\n", 2, false},
		{"short greeting", "", 1, true},
		{"short reply", "NOOP\r\n", 2, true},
	}
	for _, tc := range testCases {
		t.Run(tc.caseName, func(t *testing.T) {
			conn, client := net.Pipe()
			t.Cleanup(func() { client.Close() })
			client.SetDeadline(time.Now().Add(3 * time.Second))
			failingConn := &failingWriteConn{Conn: conn, failAt: tc.failAt, short: tc.short}
			wait := startControlSession(t, failingConn, controlSession{})
			reader := bufio.NewReader(client)
			if tc.command != "" {
				readControlReply(t, reader, "220 Service ready\r\n")
				if _, err := io.WriteString(client, tc.command); err != nil {
					t.Fatal(err)
				}
			}
			wait()
			if b, err := reader.ReadByte(); err != io.EOF {
				t.Fatalf("expected closed connection, found byte %q and error %v", b, err)
			}
		})
	}
}

func TestControlSessionHandlers(t *testing.T) {
	session := controlSession{
		Path: "/files", Type: protocolinterpreter.ASCII,
		Mode: protocolinterpreter.STREAM, Structure: protocolinterpreter.FILE,
	}
	expected := session
	reply, err := session.RouteHandler(&protocolinterpreter.FTPCommand{Token: protocolinterpreter.NOOP})
	if err != nil || reply.ToString() != "200 Command okay\r\n" || session != expected {
		t.Fatalf("NOOP changed state or returned an unexpected result: %#v, %v, %#v", reply, err, session)
	}
	reply, err = session.RouteHandler(&protocolinterpreter.FTPCommand{Token: protocolinterpreter.QUIT})
	expected.Done = true
	if err != nil || reply.ToString() != "221 Service closing control connection\r\n" || session != expected {
		t.Fatalf("QUIT did not finish session correctly: %#v, %v, %#v", reply, err, session)
	}
}
