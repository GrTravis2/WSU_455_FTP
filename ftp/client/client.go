// WSU 455 Semester Project - FTP
// FTP Client functionality
package client

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"time"

	"wsu455ftp/ftp/common"
	"wsu455ftp/ftp/constants"
	pi "wsu455ftp/ftp/protocolInterpreter"
	nvtascii "wsu455ftp/telnet/nvt-ascii"
)

type clientSession struct {
	username string
	password string
	openConn net.Conn
	reader   bufio.Reader
}

var ErrLoginFailed = errors.New("FTP login failed")
var ErrCommandRejected = errors.New("FTP command rejected")

// Create and authenticate a new client session
func NewSession(user string, pass string, ipV4 string, port string) (clientSession, error) {
	conn, err := net.DialTimeout("tcp", ipV4+":"+port, time.Second*5)
	if err != nil {
		return clientSession{}, fmt.Errorf("unable to establish connection: %v", err)
	}
	client := clientSession{username: user, password: pass, openConn: conn, reader: *bufio.NewReader(conn)}
	// check server is available, it always sends a reply when initiating a connection!
	reply, err := common.WaitForReply(client.openConn, &client.reader)
	if err != nil {
		conn.Close()
		return clientSession{}, err
	}
	if reply.Code != constants.ServiceReadyReply.Code {
		conn.Close()
		return clientSession{}, fmt.Errorf("unexpected server response: %s", reply.String())
	}

	// Make sure authentication succeeds before anything else
	err = client.login()
	if err != nil {
		conn.Close()
		return clientSession{}, ErrLoginFailed
	}

	return client, nil
}

// Send an FTP command and wait for server reply,
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
	reply, err = common.WaitForReply(client.openConn, &client.reader)
	return reply, err
}

func (client *clientSession) login() error {

	// establish connection to FTP server
	// start with sending username and checking reply
	reply, err := client.sendCmd(pi.MakeUserCmd(client.username))
	if err != nil {
		return err
	}
	if reply != constants.UsernameOkReply {
		return fmt.Errorf("unexpected server reply: %s", reply.String())
	}
	// username good, finish login by sending password
	reply, err = client.sendCmd(pi.MakePassCmd(client.password))
	if err != nil {
		return err
	}
	if reply != constants.UserLoggedInReply {
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
	if reply != constants.QuitConnectionReply {
		return fmt.Errorf("unexpected server reply: %s", reply.String())
	}
	return nil
}

func (client *clientSession) Close() error {
	return client.openConn.Close()
}

func (client *clientSession) SetPort(ipV4 string, port uint16) error {
	panic("Not Implemented!")
}

func (client *clientSession) SetType(typeCode pi.TypeCode, formatCode pi.FormatCode, byteSize uint8) error {
	reply, err := client.sendCmd(pi.MakeTypeCmd(typeCode, formatCode, byteSize))
	if err != nil {
		return err
	}
	if reply.Code >= 400 && reply.Code < 600 {
		return fmt.Errorf("%w: %s", ErrCommandRejected, reply.String())
	}
	if reply.Code != constants.CommandOkReply.Code {
		return fmt.Errorf("unexpected server reply: %s", reply.String())
	}
	return nil
}

func (client *clientSession) SetMode(code pi.ModeCode) error {
	reply, err := client.sendCmd(pi.MakeModeCmd(code))
	if err != nil {
		return err
	}
	if reply.Code >= 400 && reply.Code < 600 {
		return fmt.Errorf("%w: %s", ErrCommandRejected, reply.String())
	}
	if reply.Code != constants.CommandOkReply.Code {
		return fmt.Errorf("unexpected server reply: %s", reply.String())
	}
	return nil
}

func (client *clientSession) SetStru(code pi.StructureCode) error {
	reply, err := client.sendCmd(pi.MakeStruCmd(code))
	if err != nil {
		return err
	}
	if reply.Code >= 400 && reply.Code < 600 {
		return fmt.Errorf("%w: %s", ErrCommandRejected, reply.String())
	}
	if reply.Code != constants.CommandOkReply.Code {
		return fmt.Errorf("unexpected server reply: %s", reply.String())
	}
	return nil
}

func (client *clientSession) Retrieve(path string) error {
	panic("Not Implemented!")
}

func (client *clientSession) Store(path string) error {
	panic("Not Implemented!")
}
