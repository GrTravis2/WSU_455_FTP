// WSU 455 Semester Project - FTP
// FTP Server Tests
package server

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"
	"wsu455ftp/ftp/constants"
	pi "wsu455ftp/ftp/protocolInterpreter"
)

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

func readControlReply(t *testing.T, reader *bufio.Reader, expected string, caseName string) {
	t.Helper()
	actual, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("%s case: unable to read reply: %v", caseName, err)
	}
	if actual != expected {
		t.Fatalf("%s case: found reply %q, expected %q", caseName, actual, expected)
	}
}

func TestControlSessionLifecycle(t *testing.T) {
	conn, client := net.Pipe()
	t.Cleanup(func() { client.Close() })
	client.SetDeadline(time.Now().Add(3 * time.Second))
	wait := startControlSession(t, conn, makeDefaultControlSession())
	reader := bufio.NewReader(client)
	readControlReply(t, reader, "220 Service ready\r\n", "service ready greeting")
	if _, err := io.WriteString(client, "USER user\r\n"); err != nil {
		t.Fatal(err)
	}
	readControlReply(t, reader, constants.UsernameOkReply.String(), "user check")
	if _, err := io.WriteString(client, "PASS password\r\n"); err != nil {
		t.Fatal(err)
	}
	readControlReply(t, reader, constants.UserLoggedInReply.String(), "pass check")

	// repeated commands show that NOOP leaves the connection open
	for i := 0; i < 2; i++ {
		if _, err := io.WriteString(client, "NOOP\r\n"); err != nil {
			t.Fatal(err)
		}
		readControlReply(t, reader, "200 Command okay\r\n", "connection open")
	}
	// a parsed but unimplemented command must not produce 000
	if _, err := io.WriteString(client, "USER alice\r\n"); err != nil {
		t.Fatal(err)
	}
	readControlReply(t, reader, constants.NotLoggedInReply.String(), "user not found")

	// buffered commands after QUIT must not be processed
	if _, err := io.WriteString(client, "QUIT\r\nNOOP\r\n"); err != nil {
		t.Fatal(err)
	}
	readControlReply(t, reader, "221 Service closing control connection\r\n", "commands after quit must not be parsed")
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
				readControlReply(t, bufio.NewReader(client), "220 Service ready\r\n", caseName)
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
		{"500 reply", "UNKNOWN\r\n", 2, false},
		{"501 reply", "TYPE X\r\n", 2, false},
		{"short error reply", "QUIT extra\r\n", 2, true},
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
				readControlReply(t, reader, "220 Service ready\r\n", tc.caseName)
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

func TestControlSessionErrorRecovery(t *testing.T) {
	conn, client := net.Pipe()
	t.Cleanup(func() { client.Close() })
	client.SetDeadline(time.Now().Add(3 * time.Second))
	wait := startControlSession(t, conn, makeDefaultControlSession())
	reader := bufio.NewReader(client)
	readControlReply(t, reader, "220 Service ready\r\n", "service ready greeting")
	if _, err := io.WriteString(client, "USER user\r\n"); err != nil {
		t.Fatal(err)
	}
	readControlReply(t, reader, constants.UsernameOkReply.String(), "user check")
	if _, err := io.WriteString(client, "PASS password\r\n"); err != nil {
		t.Fatal(err)
	}
	readControlReply(t, reader, constants.UserLoggedInReply.String(), "pass check")

	var testCases = []struct {
		command  string
		reply    string
		caseName string
	}{
		{"UNKNOWN", "500 Syntax error, command unrecognized\r\n", "unknown cmd"},
		{"TYPE X", "501 Syntax error in parameters or arguments\r\n", "basic type"},
		{"MODE X", "501 Syntax error in parameters or arguments\r\n", "basic mode"},
		{"STRU X", "501 Syntax error in parameters or arguments\r\n", "basic stru"},
		{"PORT 1,2,3", "501 Syntax error in parameters or arguments\r\n", "basic port"},
		{"USER", "501 Syntax error in parameters or arguments\r\n", "user no args"},
		{"NOOP extra", "501 Syntax error in parameters or arguments\r\n", "noop extra args"},
		{"QUIT extra", "501 Syntax error in parameters or arguments\r\n", "quit extra args"},
		{"USER alice", fmt.Sprintf("%d %s\r\n", constants.NotLoggedInReply.Code, constants.NotLoggedInReply.Msg), "user not found"},
		{"PASS secret", "530 Login failed. PASS command must follow a USER command\r\n", "pass not recognized"},
		{"TYPE I", "200 Command okay\r\n", "TYPE I"},
		{"TYPE A", "200 Command okay\r\n", "TYPE A"},
		{"type a n", "200 Command okay\r\n", "type a n"},
		{"type i", "200 Command okay\r\n", "type i"},
		{"TYPE A T", "504 Command not implemented for that parameter\r\n", "TYPE A T"},
		{"TYPE A C", "504 Command not implemented for that parameter\r\n", "TYPE A C"},
		{"TYPE E", "504 Command not implemented for that parameter\r\n", "TYPE E"},
		{"TYPE E N", "504 Command not implemented for that parameter\r\n", "TYPE E N"},
		{"TYPE E T", "504 Command not implemented for that parameter\r\n", "TYPE E T"},
		{"TYPE E C", "504 Command not implemented for that parameter\r\n", "TYPE E C"},
		{"TYPE L 8", "504 Command not implemented for that parameter\r\n", "TYPE L 8"},
		{"TYPE L 16", "504 Command not implemented for that parameter\r\n", "TYPE L 16"},
		{"TYPE", "501 Syntax error in parameters or arguments\r\n", "TYPE"},
		{"TYPE A X", "501 Syntax error in parameters or arguments\r\n", "TYPE A X"},
		{"TYPE I extra", "501 Syntax error in parameters or arguments\r\n", "TYPE I extra"},
		{"TYPE L", "501 Syntax error in parameters or arguments\r\n", "TYPE L"},
		{"TYPE L 0", "501 Syntax error in parameters or arguments\r\n", "TYPE L 0"},
		{"MODE S", "200 Command okay\r\n", "MODE S"},
		{"mode s", "200 Command okay\r\n", "mode s"},
		{"MODE B", "504 Command not implemented for that parameter\r\n", "MODE B"},
		{"MODE C", "504 Command not implemented for that parameter\r\n", "MODE C"},
		{"MODE", "501 Syntax error in parameters or arguments\r\n", "MODE"},
		{"MODE S extra", "501 Syntax error in parameters or arguments\r\n", "MODE S extra"},
		{"STRU F", "200 Command okay\r\n", "STRU F"},
		{"stru f", "200 Command okay\r\n", "stru f"},
		{"STRU R", "504 Command not implemented for that parameter\r\n", "STRU R"},
		{"STRU P", "504 Command not implemented for that parameter\r\n", "STRU P"},
		{"STRU", "501 Syntax error in parameters or arguments\r\n", "STRU"},
		{"STRU F extra", "501 Syntax error in parameters or arguments\r\n", "STRU F extra"},
		{"PORT 127,0,0,1,195,80", "502 Command not implemented\r\n", "not impl"},
		{"RETR report.txt", "502 Command not implemented\r\n", "not impl"},
		{"STOR report.txt", "502 Command not implemented\r\n", "not impl"},
	}
	for _, tc := range testCases {
		// the next command must work even when both arrive in one write
		if _, err := io.WriteString(client, tc.command+"\r\nNOOP\r\n"); err != nil {
			t.Fatal(err)
		}
		readControlReply(t, reader, tc.reply, tc.caseName)
		readControlReply(t, reader, "200 Command okay\r\n", tc.caseName)
	}
	if _, err := io.WriteString(client, "QUIT\r\n"); err != nil {
		t.Fatal(err)
	}
	readControlReply(t, reader, "221 Service closing control connection\r\n", "quit session closes connection")
	if b, err := reader.ReadByte(); err != io.EOF {
		t.Fatalf("expected EOF after QUIT, found byte %q and error %v", b, err)
	}
	wait()
}

func TestControlSessionHandlers(t *testing.T) {
	session := controlSession{
		Path: "/files", Type: pi.ASCII,
		Mode: pi.STREAM, Structure: pi.FILE,
	}
	expected := session
	reply, err := session.RouteHandler(&pi.FTPCommand{Token: pi.NOOP})
	if err != nil || reply.String() != "200 Command okay\r\n" || !ControlSessionsMatch(&session, &expected) {
		t.Fatalf("NOOP changed state or returned an unexpected result: %#v, %v, %#v", reply, err, session)
	}
	reply, err = session.RouteHandler(&pi.FTPCommand{Token: pi.QUIT})
	expected.Done = true
	if err != nil || reply.String() != "221 Service closing control connection\r\n" || !ControlSessionsMatch(&session, &expected) {
		t.Fatalf("QUIT did not finish session correctly: %#v, %v, %#v", reply, err, session)
	}
}

func TestTypeHandler(t *testing.T) {
	var testCases = []struct {
		args     pi.FTPArgs
		code     uint32
		expected pi.TypeCode
		caseName string
	}{
		{pi.TypeArgs{pi.ASCII, pi.NONPRINT, 0}, 200, pi.ASCII, "ASCII Non-print"},
		{pi.TypeArgs{pi.IMAGE, pi.NONE, 0}, 200, pi.IMAGE, "Image"},
		{pi.TypeArgs{pi.ASCII, pi.TELNET, 0}, 504, pi.IMAGE, "ASCII Telnet"},
		{pi.TypeArgs{pi.ASCII, pi.CARRIAGE, 0}, 504, pi.IMAGE, "ASCII carriage control"},
		{pi.TypeArgs{pi.EBCDIC, pi.NONPRINT, 0}, 504, pi.IMAGE, "EBCDIC Non-print"},
		{pi.TypeArgs{pi.EBCDIC, pi.TELNET, 0}, 504, pi.IMAGE, "EBCDIC Telnet"},
		{pi.TypeArgs{pi.EBCDIC, pi.CARRIAGE, 0}, 504, pi.IMAGE, "EBCDIC carriage control"},
		{pi.TypeArgs{pi.LOCAL, pi.NONE, 8}, 504, pi.IMAGE, "Local 8"},
		{pi.TypeArgs{pi.LOCAL, pi.NONE, 16}, 504, pi.IMAGE, "Local 16"},
		{nil, 501, pi.IMAGE, "missing arguments"},
		{pi.ModeArgs{}, 501, pi.IMAGE, "wrong argument type"},
		{pi.TypeArgs{}, 501, pi.IMAGE, "invalid type"},
		{pi.TypeArgs{pi.ASCII, 'X', 0}, 501, pi.IMAGE, "invalid format"},
		{pi.TypeArgs{pi.ASCII, pi.NONPRINT, 8}, 501, pi.IMAGE, "ASCII with byte size"},
		{pi.TypeArgs{pi.IMAGE, pi.NONPRINT, 0}, 501, pi.IMAGE, "Image with format"},
		{pi.TypeArgs{pi.IMAGE, pi.NONE, 8}, 501, pi.IMAGE, "Image with byte size"},
		{pi.TypeArgs{pi.LOCAL, pi.NONE, 0}, 501, pi.IMAGE, "Local without byte size"},
		{pi.TypeArgs{pi.LOCAL, pi.NONPRINT, 8}, 501, pi.IMAGE, "Local with format"},
	}
	for _, tc := range testCases {
		t.Run(tc.caseName, func(t *testing.T) {
			session := controlSession{Path: "/files", Type: pi.IMAGE, Mode: pi.STREAM, Structure: pi.FILE, MockDB: map[string]string{constants.DEFAULT_USER: constants.DEFAULT_PASS}}
			if tc.code == 200 && tc.expected == pi.IMAGE {
				session.Type = pi.ASCII
			}
			expected := session
			expected.Type = tc.expected
			reply, err := session.RouteHandler(&pi.FTPCommand{Token: pi.TYPE, Args: tc.args})
			if err != nil || reply.Code != tc.code ||
				session.Path != expected.Path || session.Type != expected.Type ||
				session.Mode != expected.Mode || session.Structure != expected.Structure ||
				session.Done != expected.Done || len(session.MockDB) != 1 ||
				session.MockDB[constants.DEFAULT_USER] != constants.DEFAULT_PASS {
				t.Fatalf("unexpected TYPE result: %#v, %v, session %#v, expected %#v", reply, err, session, expected)
			}
		})
	}
}

func TestModeHandler(t *testing.T) {
	var testCases = []struct {
		args     pi.FTPArgs
		code     uint32
		expected pi.ModeCode
		caseName string
	}{
		{pi.ModeArgs{pi.STREAM}, 200, pi.STREAM, "Stream"},
		{pi.ModeArgs{pi.BLOCK}, 504, pi.STREAM, "Block"},
		{pi.ModeArgs{pi.COMPRESSED}, 504, pi.STREAM, "Compressed"},
		{nil, 501, pi.STREAM, "missing arguments"},
		{pi.TypeArgs{}, 501, pi.STREAM, "wrong argument type"},
		{pi.ModeArgs{'X'}, 501, pi.STREAM, "invalid mode"},
	}
	for _, tc := range testCases {
		t.Run(tc.caseName, func(t *testing.T) {
			session := controlSession{Path: "/files", Type: pi.IMAGE, Mode: pi.STREAM, Structure: pi.FILE, MockDB: map[string]string{constants.DEFAULT_USER: constants.DEFAULT_PASS}}
			if tc.code == 200 {
				session.Mode = pi.BLOCK
			}
			expected := session
			expected.Mode = tc.expected
			reply, err := session.RouteHandler(&pi.FTPCommand{Token: pi.MODE, Args: tc.args})
			if err != nil || reply.Code != tc.code ||
				session.Path != expected.Path || session.Type != expected.Type ||
				session.Mode != expected.Mode || session.Structure != expected.Structure ||
				session.Done != expected.Done || len(session.MockDB) != 1 ||
				session.MockDB[constants.DEFAULT_USER] != constants.DEFAULT_PASS {
				t.Fatalf("unexpected MODE result: %#v, %v, session %#v, expected %#v", reply, err, session, expected)
			}
		})
	}
}

func TestStructureHandler(t *testing.T) {
	var testCases = []struct {
		args     pi.FTPArgs
		code     uint32
		expected pi.StructureCode
		caseName string
	}{
		{pi.StructureArgs{pi.FILE}, 200, pi.FILE, "File"},
		{pi.StructureArgs{pi.RECORD}, 504, pi.FILE, "Record"},
		{pi.StructureArgs{pi.PAGE}, 504, pi.FILE, "Page"},
		{nil, 501, pi.FILE, "missing arguments"},
		{pi.ModeArgs{}, 501, pi.FILE, "wrong argument type"},
		{pi.StructureArgs{'X'}, 501, pi.FILE, "invalid structure"},
	}
	for _, tc := range testCases {
		t.Run(tc.caseName, func(t *testing.T) {
			session := controlSession{Path: "/files", Type: pi.IMAGE, Mode: pi.STREAM, Structure: pi.FILE, MockDB: map[string]string{constants.DEFAULT_USER: constants.DEFAULT_PASS}}
			if tc.code == 200 {
				session.Structure = pi.RECORD
			}
			expected := session
			expected.Structure = tc.expected
			reply, err := session.RouteHandler(&pi.FTPCommand{Token: pi.STRU, Args: tc.args})
			if err != nil || reply.Code != tc.code ||
				session.Path != expected.Path || session.Type != expected.Type ||
				session.Mode != expected.Mode || session.Structure != expected.Structure ||
				session.Done != expected.Done || len(session.MockDB) != 1 ||
				session.MockDB[constants.DEFAULT_USER] != constants.DEFAULT_PASS {
				t.Fatalf("unexpected STRU result: %#v, %v, session %#v, expected %#v", reply, err, session, expected)
			}
		})
	}
}

// Testing Helpers

// Compare two control sessions, return true if all values outside of the mock
// database map are the same, false otherwise.
func ControlSessionsMatch(lhs *controlSession, rhs *controlSession) bool {
	match := (lhs.Path == rhs.Path &&
		lhs.Type == rhs.Type &&
		lhs.Mode == rhs.Mode &&
		lhs.Structure == rhs.Structure &&
		lhs.Done == rhs.Done)

	return match
}
