// WSU 455 Semester Project - FTP
// Functions which are used across the project
// for common utility purposes.
package common

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"time"
	pi "wsu455ftp/ftp/protocolInterpreter"
	nvtascii "wsu455ftp/telnet/nvt-ascii"
)

/* Constanst */

// Timeout for waiting on replies
const READ_TIMEOUT = 15 * time.Second

func NotImplementedError() {
	panic("not implemented!")
}

// format and send a reply over the control connection
func SendReply(conn net.Conn, reply pi.FTPReply) error {
	b, err := nvtascii.FromString(reply.String())
	if err != nil {
		return err
	}
	n, err := conn.Write(b)
	if err != nil {
		return err
	}
	if n != len(b) {
		return io.ErrShortWrite
	}
	return nil
}

func WaitForReply(conn net.Conn, reader *bufio.Reader) (pi.FTPReply, error) {
	var reply pi.FTPReply = pi.FTPReply{}
	var err error = nil
	// read from control connection with timeout
	err = conn.SetReadDeadline(time.Now().Add(READ_TIMEOUT))
	if err != nil {
		err = fmt.Errorf("Failed to set read timeout: %v", err)
	}
	msg, err := reader.ReadString('\n')
	if err != nil {
		err = fmt.Errorf("Transmission failed: %v", err)
	}
	reply, err = pi.ParseFTPReply(msg)
	if err != nil {
		err = fmt.Errorf("Unable to parse server reply: %v", err)
	}
	return reply, err
}
