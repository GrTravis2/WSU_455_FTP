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
	"wsu455ftp/ftp/common"
	"wsu455ftp/ftp/constants"
	pi "wsu455ftp/ftp/protocolInterpreter"
)

// Session state used to apply commands
type controlSession struct {
	Path      string
	Type      pi.TypeCode
	Mode      pi.ModeCode
	Structure pi.StructureCode
	Done      bool
	MockDB    map[string]string
}

func makeDefaultControlSession() controlSession {
	return controlSession{
		Path:      "",
		Type:      pi.ASCII,
		Mode:      pi.STREAM,
		Structure: pi.FILE,
		Done:      false,
		MockDB:    map[string]string{constants.DEFAULT_USER: constants.DEFAULT_PASS},
	}
}

// FTP Server Entry Point
func Run(ctx context.Context, ipV4 string, port string) error {
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
		default_session := makeDefaultControlSession()

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

/*
Handle an error related to FTP interpret, returning a bool which
represents if the error should terminate the session if true
*/
func handleInterpretError(err error, conn net.Conn) bool {
	var reply pi.FTPReply
	switch {
	case errors.Is(err, pi.ErrInvalidCommand):
		reply = pi.MakeFTPReply(500, "Syntax error, command unrecognized")
	case errors.Is(err, pi.ErrInvalidArguments):
		reply = pi.MakeFTPReply(501, "Syntax error in parameters or arguments")
	default:
		fmt.Fprintf(os.Stderr, "Unable to interpret command: %v", err)
		return true
	}
	if sendErr := common.SendReply(conn, reply); sendErr != nil {
		fmt.Fprintf(os.Stderr, "Unable to send error reply: %v", err)
		return true
	}
	return false
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
	if err := common.SendReply(openConn, constants.ServiceReadyReply); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to send greeting: %v", err)
		return
	}
	// new session always starts with USER then PASS commands
	reader := bufio.NewReader(openConn)
	authenticated := session.authenticateUser(openConn, reader)
	if !authenticated {
		// sessions terminating dont handle errs, send and return
		common.SendReply(openConn, constants.NotLoggedInReply)
		return
	}
	common.SendReply(openConn, constants.UserLoggedInReply)

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
			exit := handleInterpretError(err, openConn)
			if exit {
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
			if err := common.SendReply(openConn, reply); err != nil {
				fmt.Fprintf(os.Stderr, "Unable to send reply: %v", err)
				return
			}
			if session.Done {
				return
			}
		}
	}
}

// Authenticate a client with a valid USER and PASS command, return true if
// login successful, false otherwise.
func (session *controlSession) authenticateUser(conn net.Conn, reader *bufio.Reader) bool {
	msg, err := reader.ReadString('\n')
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading client command: %v", err)
		return false
	}
	// first cmd must be a USER cmd with user in mockDB
	cmds, err := pi.Interpret(msg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing client command: %v", err)
		return false
	}
	if len(cmds) > 1 || cmds[0].Token != pi.USER {
		fmt.Fprintf(os.Stderr, "client should start with a single USER command: %v", err)
		return false
	}
	userArgs, _ := cmds[0].Args.(pi.UserArgs)
	user := userArgs.Username
	pass, found := session.MockDB[user]
	if !found { // user not recognized
		return false
	}
	common.SendReply(conn, constants.UsernameOkReply)
	msg, err = reader.ReadString('\n')
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading client command: %v", err)
		return false
	}
	// second cmd must be a PASS cmd with correct password from mockDB
	cmds, err = pi.Interpret(msg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing client command: %v", err)
		return false
	}
	if len(cmds) > 1 || cmds[0].Token != pi.PASS {
		fmt.Fprintf(os.Stderr, "client must follow a USER command with a single PASS command: %v", err)
		return false
	}
	passArgs, _ := cmds[0].Args.(pi.PassArgs)
	if pass != passArgs.Password {
		return false
	}
	return true
}

// Command Handlers

// Call handler for each specific FTP Command
func (session *controlSession) RouteHandler(cmd *pi.FTPCommand) (pi.FTPReply, error) {
	var reply pi.FTPReply
	var err error = nil
	switch cmd.Token {
	case pi.USER:
		// TODO: FTP supports changing users at any time, update
		// This switch statement to change user AND THEN look for password
		args, _ := cmd.Args.(pi.UserArgs)
		reply, err = session.UserHandler(&args)
	case pi.PASS:
		reply = pi.MakeFTPReply(530, "Login failed. PASS command must follow a USER command")
		err = nil
	case pi.QUIT:
		reply, err = session.QuitHandler()
	case pi.PORT:
		args, _ := cmd.Args.(pi.PortArgs)
		reply, err = session.PortHandler(&args)
	case pi.TYPE:
		args, _ := cmd.Args.(pi.TypeArgs)
		reply, err = session.TypeHandler(&args)
	case pi.MODE:
		args, _ := cmd.Args.(pi.ModeArgs)
		reply, err = session.ModeHandler(&args)
	case pi.STRU:
		args, _ := cmd.Args.(pi.StructureArgs)
		reply, err = session.StructureHandler(&args)
	case pi.RETR:
		args, _ := cmd.Args.(pi.RetrArgs)
		reply, err = session.RetrieveHandler(&args)
	case pi.STOR:
		args, _ := cmd.Args.(pi.StorArgs)
		reply, err = session.StoreHandler(&args)
	case pi.NOOP:
		reply, err = session.NOOPHandler()
	default:
		reply = pi.MakeFTPReply(502, "Command not implemented")
	}

	// handlers which are still stubs must not send a 000 reply
	if err == nil && reply.Code == 0 {
		reply = pi.MakeFTPReply(502, "Command not implemented")
	}
	return reply, err
}

// handle USER command
func (session *controlSession) UserHandler(args *pi.UserArgs) (pi.FTPReply, error) {
	_, found := session.MockDB[args.Username]
	if !found {
		return constants.NotLoggedInReply, nil
	}
	return constants.UsernameOkReply, nil
}

// handle PASS command
func (session *controlSession) PassHandler(user string, args *pi.PassArgs) (pi.FTPReply, error) {
	pass := session.MockDB[user]
	if pass != args.Password {
		return constants.NotLoggedInReply, nil
	}
	return constants.UserLoggedInReply, nil
}

// handle QUIT command
func (session *controlSession) QuitHandler() (pi.FTPReply, error) {
	session.Done = true
	return constants.QuitConnectionReply, nil
}

// handle PORT command
func (session *controlSession) PortHandler(cmd *pi.PortArgs) (pi.FTPReply, error) {
	return pi.FTPReply{}, nil
}

// handle Type command
func (session *controlSession) TypeHandler(cmd *pi.TypeArgs) (pi.FTPReply, error) {
	return pi.FTPReply{}, nil
}

// handle Mode command
func (session *controlSession) ModeHandler(cmd *pi.ModeArgs) (pi.FTPReply, error) {
	return pi.FTPReply{}, nil
}

// handle Structure command
func (session *controlSession) StructureHandler(cmd *pi.StructureArgs) (pi.FTPReply, error) {
	return pi.FTPReply{}, nil
}

// handle Retrieve command
func (session *controlSession) RetrieveHandler(cmd *pi.RetrArgs) (pi.FTPReply, error) {
	return pi.FTPReply{}, nil
}

// handle Store command
func (session *controlSession) StoreHandler(cmd *pi.StorArgs) (pi.FTPReply, error) {
	return pi.FTPReply{}, nil
}

// handle NOOP command
func (session *controlSession) NOOPHandler() (pi.FTPReply, error) {
	return constants.CommandOkReply, nil
}
