// WSU 455 Semester Project - FTP
// FTP Server functionality
package server

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"sync"
	pi "wsu455ftp/ftp/protocolInterpreter"
	nvtascii "wsu455ftp/telnet/nvt-ascii"
)

const DEFAULT_CONTROL_PORT string = "21"
const DEFAULT_DATA_PORT string = "20"

/* Constant Replies */
var ServiceReadyReply = pi.MakeFTPReply(220, "Service ready")
var UsernameOkReply = pi.MakeFTPReply(331, "User name ok, need password")
var UserLoggedInReply = pi.MakeFTPReply(230, "User logged in")

// Session state used to apply commands
type controlSession struct {
	Path      string
	Type      pi.TypeCode
	Mode      pi.ModeCode
	Structure pi.StructureCode
	Done      bool
}

// FTP Server Entry Point
func run(ctx context.Context, ipV4 string, port string) error {
	// register passed context to end on interrupt
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()
	var serverAcceptingWg sync.WaitGroup // for tracking that server is done accepting new conns
	var cleanupWg sync.WaitGroup         // for tracking that there are no active sessions left

	// create the FTP server lisenting on ip:port
	server, err := net.Listen("tcp4", ipV4+":"+port)
	if err != nil {
		return fmt.Errorf("Unable to start control server: %v", err)
	}

	// spin off a thread to listen and handle control sessions
	serverAcceptingWg.Add(1)                       //
	active_sessions := make(map[net.Conn]struct{}) // track active sessino objects
	var mutex sync.Mutex                           // lock for modifying sessions map
	go func() {
		defer serverAcceptingWg.Done()
		default_session := controlSession{
			Path:      "",
			Type:      pi.ASCII,
			Mode:      pi.STREAM,
			Structure: pi.FILE,
			Done:      false,
		}

		// serve forever until parent context ends
		for {
			// listen and wait for a client to connect
			conn, err := server.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) { // server closed, exit goroutine
					return
				} else { // actual err
					fmt.Fprintf(os.Stderr, "Connection failed: %v", err)
				}
			} else {
				// spin off a control session thread
				mutex.Lock()
				active_sessions[conn] = struct{}{} // add reference to active sessions
				cleanupWg.Add(1)
				mutex.Unlock()
				go controlSessionHandler(conn, default_session, &cleanupWg, &mutex, active_sessions)
			}
		}
	}()

	// spin off a thread to shutdown when parent context ends
	cleanupWg.Add(1) //
	func() {
		defer cleanupWg.Done()
		<-ctx.Done() // block until ctx is ended then run shutdown logic

		// close the server and then close all active connections
		if err := server.Close(); err != nil && err != net.ErrClosed {
			fmt.Fprintf(os.Stderr, "error listening and serving: %s\n", err)
		}
		serverAcceptingWg.Wait() // wait until server is no longer accepting conns
		mutex.Lock()             // cleanup active conns
		for conn := range active_sessions {
			conn.Close()
		}
		mutex.Unlock()
	}()
	cleanupWg.Wait() // wait here until shutdown thread complete
	return nil
}

func controlSessionHandler(
	openConn net.Conn,
	session controlSession,
	wg *sync.WaitGroup,
	mutex *sync.Mutex,
	active_sessions map[net.Conn]struct{},
) {
	defer wg.Done()
	defer func() {
		openConn.Close()
		mutex.Lock()
		delete(active_sessions, openConn)
		mutex.Unlock()
	}()
	// send server greeting here to kick off control session (clients are expecting it)
	if err := sendReply(openConn, ServiceReadyReply); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to send greeting: %v", err)
		return
	}

	reader := bufio.NewReader(openConn)

	// read until EOF found or connection fails
	for {
		msg, err := reader.ReadString('\n')

		// if err is EOF or connection fails, just end
		if err != nil {
			if err == io.EOF { // EOF found, connection is complete
				fmt.Print("Transmission complete.")
			} else {
				fmt.Fprintf(os.Stderr, "Transmission failed: %v", err)
			}
			return
		}
		cmds, err := pi.Interpret(msg)
		if err != nil {
			var reply pi.FTPReply
			switch {
			case errors.Is(err, pi.ErrInvalidCommand):
				reply = pi.MakeFTPReply(500, "Syntax error, command unrecognized")
			case errors.Is(err, pi.ErrInvalidArguments):
				reply = pi.MakeFTPReply(501, "Syntax error in parameters or arguments")
			default:
				fmt.Fprintf(os.Stderr, "Unable to interpret command: %v", err)
				return
			}
			if err := sendReply(openConn, reply); err != nil {
				fmt.Fprintf(os.Stderr, "Unable to send error reply: %v", err)
				return
			}
			continue
		}

		// for each parsed command, action it and then send reply back
		for _, c := range cmds {
			reply, err := session.RouteHandler(&c)
			if err != nil { // handle err during cmd handling
				fmt.Fprintf(os.Stderr, "Failed to handle command %v: %v", c.Token, err)
				return
			}
			if err := sendReply(openConn, reply); err != nil {
				fmt.Fprintf(os.Stderr, "Unable to send reply: %v", err)
				return
			}
			if session.Done {
				return
			}
		}
	}
}

// format and send a reply over the control connection
func sendReply(conn net.Conn, reply pi.FTPReply) error {
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

// Command Handlers

// Call handler for each specific FTP Command
func (session *controlSession) RouteHandler(cmd *pi.FTPCommand) (pi.FTPReply, error) {
	var reply pi.FTPReply
	var err error = nil
	switch cmd.Token {
	case pi.USER:
		reply, err = session.UserHandler(cmd)
	case pi.PASS:
		reply, err = session.PassHandler(cmd)
	case pi.QUIT:
		reply, err = session.QuitHandler(cmd)
	case pi.PORT:
		reply, err = session.PortHandler(cmd)
	case pi.TYPE:
		reply, err = session.TypeHandler(cmd)
	case pi.MODE:
		reply, err = session.ModeHandler(cmd)
	case pi.STRU:
		reply, err = session.StructureHandler(cmd)
	case pi.RETR:
		reply, err = session.RetrieveHandler(cmd)
	case pi.STOR:
		reply, err = session.StoreHandler(cmd)
	case pi.NOOP:
		reply, err = session.NOOPHandler(cmd)
	default:
		return pi.MakeFTPReply(502, "Command not implemented"), nil
	}

	// handlers which are still stubs must not send a 000 reply
	if err == nil && reply.Code == 0 {
		reply = pi.MakeFTPReply(502, "Command not implemented")
	}
	return reply, err
}

// handle USER command
func (session *controlSession) UserHandler(cmd *pi.FTPCommand) (pi.FTPReply, error) {
	return pi.FTPReply{}, nil
}

// handle PASS command
func (session *controlSession) PassHandler(cmd *pi.FTPCommand) (pi.FTPReply, error) {
	return pi.FTPReply{}, nil
}

// handle QUIT command
func (session *controlSession) QuitHandler(cmd *pi.FTPCommand) (pi.FTPReply, error) {
	session.Done = true
	return pi.MakeFTPReply(221, "Service closing control connection"), nil
}

// handle PORT command
func (session *controlSession) PortHandler(cmd *pi.FTPCommand) (pi.FTPReply, error) {
	return pi.FTPReply{}, nil
}

// handle Type command
func (session *controlSession) TypeHandler(cmd *pi.FTPCommand) (pi.FTPReply, error) {
	return pi.FTPReply{}, nil
}

// handle Mode command
func (session *controlSession) ModeHandler(cmd *pi.FTPCommand) (pi.FTPReply, error) {
	return pi.FTPReply{}, nil
}

// handle Structure command
func (session *controlSession) StructureHandler(cmd *pi.FTPCommand) (pi.FTPReply, error) {
	return pi.FTPReply{}, nil
}

// handle Retrieve command
func (session *controlSession) RetrieveHandler(cmd *pi.FTPCommand) (pi.FTPReply, error) {
	return pi.FTPReply{}, nil
}

// handle Store command
func (session *controlSession) StoreHandler(cmd *pi.FTPCommand) (pi.FTPReply, error) {
	return pi.FTPReply{}, nil
}

// handle NOOP command
func (session *controlSession) NOOPHandler(cmd *pi.FTPCommand) (pi.FTPReply, error) {
	return pi.MakeFTPReply(200, "Command okay"), nil
}
