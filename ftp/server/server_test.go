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

const ServerIP string = "127.0.0.1"
const ServerPort string = "5678"

// start a control session and check cleanup when it ends
func startControlSession(t *testing.T, session controlSession) func() {
	t.Helper()
	active_sessions := map[net.Conn]struct{}{session.controlConn: {}}
	var mutex sync.Mutex
	var wg sync.WaitGroup
	wg.Add(1)
	go controlSessionHandler(session, &wg, &mutex, active_sessions)
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
		session.controlConn.Close()
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
	wait := startControlSession(t, makeDefaultControlSession(conn, ServerIP, ServerPort))
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
			wait := startControlSession(t, makeDefaultControlSession(conn, ServerIP, ServerPort))
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
			wait := startControlSession(t, makeDefaultControlSession(failingConn, ServerIP, ServerPort))
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
	wait := startControlSession(t, makeDefaultControlSession(conn, ServerIP, ServerPort))
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
		{"TYPE A", "502 Command not implemented\r\n", "not impl"},
		{"TYPE E N", "502 Command not implemented\r\n", "not impl"},
		{"TYPE I", "502 Command not implemented\r\n", "not impl"},
		{"TYPE L 8", "502 Command not implemented\r\n", "not impl"},
		{"MODE S", "502 Command not implemented\r\n", "not impl"},
		{"STRU F", "502 Command not implemented\r\n", "not impl"},
		// {"PORT 127,0,0,1,195,80", "502 Command not implemented\r\n", "not impl"},
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

// Temp file Helper
func TempFileHelper(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.txt")
	err := os.WriteFile(path, data, 0o644)
	if (err != nil){t.Fatal(err)}
	return path
}

// payloadGen
func MakePayload(size int) []byte{
	rng := rand.New(rand.NewSource(1))
	data := make([]byte, size)
	rng.Read(data)
	if size >= 4 {
		data[0] = '\r'
		data[1] = '\n'
		data[2] = 0
		data[size-1] = 0xFF
	}
	return data
}

// run-retrieve Helper
// this will connect a fake client to the retrievehelper
func RunRetrieveHelper(t *testing.T, path string, fileType pi.TypeCode) []byte {
	t.Helper()
	server, client := net.Pipe()
	server.SetDeadline(time.Now().Add(3 * time.Second))
	client.SetDeadline(time.Now().Add(3 * time.Second))
	var reciever []byte
	var readError error
	var endChannel make(chan struct{})
	go func(){
		reciever, readError = io.ReadAll(client)
		close(endChannel)
	}()
	helpErr := retrieveHelper(server, path, fileType, pi.STREAM, pi.FILE)
	server.Close()
	<- endChannel
	if readError != nil {
		t.Fatal(readError)
	}
	client.Close()
	return reciever, helpErr
}

// helper to check that the bytes recieved match the bytes expected
// on a missmatch it will report where they differ without printing the contents
func CompareBytesHelper(t *testing.T, recieved []byte, wanted []byte, caseFail string) {
	t.Helper()
	if bytes.Equal(recieved, wanted) {
		return;
	}
	counter := 0
	for counter < len(recieved) && counter < len(wanted) && recieved[counter] == wanted[counter] {
		counter++
	}
	t.Fatalf("%s Case: recieved %d total bytes, wanted %d byte amount, first mismatch occured at index %d", caseFail, len(recieved), len(wanted), counter)
}

// this test is for verifying retrieve is able to retrieve a small file properly.
func TestRetrieveHelperSmallFile(t *testing.T) {
	testString := []byte("test line one\ntest line two\n")
	path := TempFileHelper(t, testString)
	retrieved, err := RunRetrieveHelper(t, path, pi.ASCII)
	if err != nil {
		t.Fatalf("%v error occured", err)
	}
	CompareBytesHelper(t, retrieved, testString, "small file")
}

// this test is for verifying that an empty file is retrieved as zero bytes
func TestRetrieveHelperEmptyFile(t *testing.T) {
	testString := []byte("")
	path := TempFileHelper(t, testString)
	retrieved, err := RunRetrieveHelper(t, path, pi.ASCII)
	if err != nil {
		t.Fatalf("%v error occured", err)
	}
	CompareBytesHelper(t, retrieved, testString, "empty file")
}

// this test is for verifying that binary data is loading, this includes \r, \n, NUL, and 0xFF is retrieved byte for byte.
func TestRetrieveHelperBinaryFile(t *testing.T) {
	testdata := MakePayload(300)
	path := TempFileHelper(t, testdata)
	retrieved, err := RunRetrieveHelper(t, path, pi.ASCII)
	if err != nil {
		t.Fatalf("%v error occured", err)
	}
	CompareBytesHelper(t, retrieved, testdata, "binary file")
}

// this test is for verifying that the retrievehelper is able to process large files.
func TestRetrieveHelperLargeFile(t *testing.T) {
	testdata := MakePayload(4*1024*1024 + 133)
	path := TempFileHelper(t, testdata)
	retrieved, err := RunRetrieveHelper(t, path, pi.ASCII)
	if err != nil {
		t.Fatalf("%v error occured", err)
	}
	CompareBytesHelper(t, retrieved, testdata, "large file")
}