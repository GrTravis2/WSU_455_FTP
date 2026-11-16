// WSU 455 Semester Project - FTP
// FTP Client functionality
package client

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	pi "wsu455ftp/ftp/protocolInterpreter"
	"wsu455ftp/ftp/server"
	nvtascii "wsu455ftp/telnet/nvt-ascii"
)

type clientSession struct {
	username string
	password string
	openConn net.Conn
	reader   bufio.Reader
}

var ErrLoginFailed = errors.New("FTP login failed")

// Create and authenticate a new client session
func NewSession(user string, pass string, ipV4 string, port string) (clientSession, error) {
	conn, err := net.DialTimeout("tcp", ipV4+":"+port, time.Second*5)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nable to establish connection: %v", err)
	}
	client := clientSession{username: user, password: pass, openConn: conn, reader: *bufio.NewReader(conn)}
	// check server is available, it always sends a reply when initiating a connection!
	msg, err := client.reader.ReadString('\n')
	if err != nil {
		return clientSession{}, fmt.Errorf("Transmission failed: %v", err)
	}
	parsedReply, err := pi.ParseFTPReply(msg)
	if err != nil {
		return clientSession{}, fmt.Errorf("unable to parse server reply: %v", err)
	}
	if parsedReply != server.ServiceReadyReply {
		return clientSession{}, fmt.Errorf("unexpected server response: %s", parsedReply.String())
	}

	// Make sure authentication succeeds before anything else
	err = client.login()
	if err != nil {
		return clientSession{}, ErrLoginFailed
	}

	return client, nil
}

func (client *clientSession) login() error {

	// establish connection to FTP server
	// start with sending username and checking reply
	cmd, _ := nvtascii.FromString(pi.MakeUserCmd(client.username).String() + nvtascii.CRLF)
	_, err := client.openConn.Write(cmd) // TODO
	if err != nil {
		return err
	}
	msg, err := client.reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("Transmission failed: %v", err)
	}
	parsedReply, err := pi.ParseFTPReply(msg)
	if err != nil {
		return fmt.Errorf("unable to parse server reply: %v", err)
	}
	if parsedReply != server.UsernameOkReply {
		return fmt.Errorf("unexpected server reply: %s", parsedReply)
	}

	// username good, finish login by sending password
	cmd, _ = nvtascii.FromString(pi.MakePassCmd(client.password).String() + nvtascii.CRLF)
	_, err = client.openConn.Write(cmd) // TODO
	if err != nil {
		return err
	}
	msg, err = client.reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("Transmission failed: %v", err)
	}
	parsedReply, err = pi.ParseFTPReply(msg)
	if err != nil {
		return fmt.Errorf("unable to parse server reply: %v", err)
	}
	if parsedReply != server.UserLoggedInReply {
		return fmt.Errorf("unexpected server reply: %s", parsedReply)
	}

	return nil
}
