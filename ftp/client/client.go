// WSU 455 Semester Project - FTP
// FTP Client functionality
package client

import (
	"fmt"
	"net"
	"os"
	"time"

	pi "wsu455ftp/ftp/protocolInterpreter"
	nvtascii "wsu455ftp/telnet/nvt-ascii"
)

type clientSession struct {
	username string
	password string
}

func run(user string, pass string, ipV4 string, port string) error {

	// establish connection to FTP server
	conn, err := net.DialTimeout("tcp", ipV4, time.Second*5)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to establish connection: %v", err)
	}
	cmd, err := nvtascii.FromString(pi.MakeUserCmd(user).String())
	_, err = conn.Write(cmd) // TODO

	return nil
}
