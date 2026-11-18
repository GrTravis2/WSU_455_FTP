// WSU 455 Semester Project - FTP
// FTP Client functionality
package client

import (
	"bufio"
	"errors"
	"fmt"
	"net"
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
		return clientSession{}, fmt.Errorf("unable to establish connection: %v", err)
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

func (client *clientSession) sendCmd(cmd pi.FTPCommand) (pi.FTPReply, error) {
	var reply pi.FTPReply
	msg, err := nvtascii.FromString(cmd.String() + nvtascii.CRLF)
	if err != nil {
		return pi.FTPReply{}, fmt.Errorf("unable to convert FTP cmd to nvt-ascii: %v", err)
	}
	_, err = client.openConn.Write(msg)
	if err != nil {
		return pi.FTPReply{}, err
	}
	response, err := client.reader.ReadString('\n')
	if err != nil {
		return pi.FTPReply{}, fmt.Errorf("Transmission failed: %v", err)
	}
	reply, err = pi.ParseFTPReply(response)
	if err != nil {
		return pi.FTPReply{}, fmt.Errorf("unable to parse server reply: %v", err)
	}
	return reply, nil
}

func (client *clientSession) login() error {

	// establish connection to FTP server
	// start with sending username and checking reply
	reply, err := client.sendCmd(pi.MakeUserCmd(client.username))
	if err != nil {
		return err
	}
	if reply != server.UsernameOkReply {
		return fmt.Errorf("unexpected server reply: %s", reply.String())
	}
	// username good, finish login by sending password
	reply, err = client.sendCmd(pi.MakePassCmd(client.password))
	if err != nil {
		return err
	}
	if reply != server.UserLoggedInReply {
		return fmt.Errorf("unexpected server reply: %s", reply.String())
	}
	return nil
}

func (client *clientSession) Quit() error {
	defer client.openConn.Close()
	reply, err := client.sendCmd(pi.MakeQuitCmd())
	if err != nil {
		return err
	}
	if reply != server.QuitConnectionReply {
		return fmt.Errorf("unexpected server reply: %s", reply.String())
	}
	return nil
}
