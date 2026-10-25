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
	protocolinterpreter "wsu455ftp/ftp/protocolInterpreter"
	nvtascii "wsu455ftp/telnet/nvt-ascii"
)

const DEFAULT_CONTROL_PORT string = "21"
const DEFAULT_DATA_PORT string = "20"

// Session state used to apply commands
type controlSession struct {
	Path      string
	Type      protocolinterpreter.TypeCode
	Mode      protocolinterpreter.ModeCode
	Structure protocolinterpreter.StructureCode
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
			Type:      protocolinterpreter.ASCII,
			Mode:      protocolinterpreter.STREAM,
			Structure: protocolinterpreter.FILE,
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
	defer openConn.Close()
	defer wg.Done()
	// send server greeting here to kick off control session (clients are expecting it)

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
			// remove from active sessions before returning
			mutex.Lock()
			delete(active_sessions, openConn)
			mutex.Unlock()
			return
		}
		cmds, err := protocolinterpreter.Interpret(msg)
		if err != nil {
			// handle parse failure -> send failure reply to client and try again?
		}

		// for each parsed command, action it and then send reply back
		for _, c := range cmds {
			reply, err := session.RouteHandler(&c)
			if err != nil { // handle err during cmd handling
				fmt.Fprintf(os.Stderr, "Failed to handle command %v: %v", c.Token, err)
			}
			b, err := nvtascii.FromString(reply.ToString())
			if err != nil {
				fmt.Fprintf(os.Stderr, "Unable to convert string to NVT ASCII: %v", err)
			}
			openConn.Write(b)
		}
	}
}

// Command Handlers

// Call handler for each specific FTP Command
func (session *controlSession) RouteHandler(cmd *protocolinterpreter.FTPCommand) (FTPReply, error) {
	var reply FTPReply
	var err error = nil
	switch cmd.Token {
	case protocolinterpreter.USER:
		session.UserHandler(cmd)
	case protocolinterpreter.PASS:
		session.PassHandler(cmd)
	case protocolinterpreter.QUIT:
		session.QuitHandler(cmd)
	case protocolinterpreter.PORT:
		session.PortHandler(cmd)
	case protocolinterpreter.TYPE:
		session.TypeHandler(cmd)
	case protocolinterpreter.MODE:
		session.ModeHandler(cmd)
	case protocolinterpreter.STRU:
		session.StructureHandler(cmd)
	case protocolinterpreter.RETR:
		session.RetrieveHandler(cmd)
	case protocolinterpreter.STOR:
		session.StoreHandler(cmd)
	case protocolinterpreter.NOOP:
		session.NOOPHandler(cmd)
	default:
		fmt.Fprintf(os.Stderr, "Unable to handle unrecognized command: %v", cmd.Token)
	}

	return reply, err
}

// handle USER command
func (session *controlSession) UserHandler(cmd *protocolinterpreter.FTPCommand) (FTPReply, error) {
	return FTPReply{}, nil
}

// handle PASS command
func (session *controlSession) PassHandler(cmd *protocolinterpreter.FTPCommand) (FTPReply, error) {
	return FTPReply{}, nil
}

// handle QUIT command
func (session *controlSession) QuitHandler(cmd *protocolinterpreter.FTPCommand) (FTPReply, error) {
	return FTPReply{}, nil
}

// handle PORT command
func (session *controlSession) PortHandler(cmd *protocolinterpreter.FTPCommand) (FTPReply, error) {
	return FTPReply{}, nil
}

// handle Type command
func (session *controlSession) TypeHandler(cmd *protocolinterpreter.FTPCommand) (FTPReply, error) {
	return FTPReply{}, nil
}

// handle Mode command
func (session *controlSession) ModeHandler(cmd *protocolinterpreter.FTPCommand) (FTPReply, error) {
	return FTPReply{}, nil
}

// handle Structure command
func (session *controlSession) StructureHandler(cmd *protocolinterpreter.FTPCommand) (FTPReply, error) {
	return FTPReply{}, nil
}

// handle Retrieve command
func (session *controlSession) RetrieveHandler(cmd *protocolinterpreter.FTPCommand) (FTPReply, error) {
	return FTPReply{}, nil
}

// handle Store command
func (session *controlSession) StoreHandler(cmd *protocolinterpreter.FTPCommand) (FTPReply, error) {
	return FTPReply{}, nil
}

// handle NOOP command
func (session *controlSession) NOOPHandler(cmd *protocolinterpreter.FTPCommand) (FTPReply, error) {
	return FTPReply{}, nil
}

type FTPReply struct {
	Code [3]byte
	Msg  string
}

func (reply *FTPReply) ToString() string {
	return fmt.Sprintf("%d%d%d %s\n\n", reply.Code[0], reply.Code[1], reply.Code[2], reply.Msg)
}
