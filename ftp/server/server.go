// WSU 455 Semester Project - FTP
// FTP Server functionality
package server

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"time"
	"wsu455ftp/ftp/common"
	"wsu455ftp/ftp/constants"
	pi "wsu455ftp/ftp/protocolInterpreter"
)

// Session state used to apply commands
type controlSession struct {
	controlConn net.Conn
	Path        string
	Type        pi.TypeCode
	Mode        pi.ModeCode
	Structure   pi.StructureCode
	Done        bool
	MockDB      map[string]string
	serverIp    string
	serverPort  string
	clientIP    string
	clientPort  string
	dataChan    chan transferJob
	dataIP      string
	dataPort    string
}

type transferJob struct {
	fileOp   pi.FTPTokenType
	filePath string
	reply    pi.FTPReply
}

func makeDefaultControlSession(
	controlConn net.Conn,
	serverIP string,
	serverPort string,
) controlSession {
	return controlSession{
		controlConn: controlConn,
		Path:        "",
		Type:        pi.ASCII,
		Mode:        pi.STREAM,
		Structure:   pi.FILE,
		Done:        false,
		MockDB:      map[string]string{constants.DEFAULT_USER: constants.DEFAULT_PASS},
		dataChan:    make(chan transferJob, 32),
		serverIp:    serverIP,
		serverPort:  serverPort,
	}
}

// FTP Server Entry Point
func Run(
	ctx context.Context,
	serverIP string,
	serverPort string,
) error {
	// register passed context to end on interrupt
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()
	var serverAcceptingWg sync.WaitGroup // for tracking that server is done accepting new conns
	var cleanupWg sync.WaitGroup         // for tracking that there are no active sessions left

	// create the FTP server lisenting on ip:port
	server, err := net.Listen("tcp4", serverIP+":"+serverPort)
	if err != nil {
		return fmt.Errorf("Unable to start control server: %v", err)
	}

	// spin off a thread to listen and handle control sessions
	serverAcceptingWg.Add(1)                       //
	active_sessions := make(map[net.Conn]struct{}) // track active sessino objects
	var mutex sync.Mutex                           // lock for modifying sessions map
	go func() {
		defer serverAcceptingWg.Done()

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
				default_session := makeDefaultControlSession(conn, serverIP, serverPort)
				mutex.Lock()
				active_sessions[conn] = struct{}{} // add reference to active sessions
				cleanupWg.Add(1)
				mutex.Unlock()
				ip, port, found := strings.Cut(conn.RemoteAddr().String(), ":")
				if !found {
					fmt.Fprintf(os.Stderr, "Expected IPv4 address found: %v", conn.LocalAddr().String())
					continue // skip non-ipv4 strings
				}
				default_session.clientIP = ip
				default_session.clientPort = port
				go controlSessionHandler(default_session, &cleanupWg, &mutex, active_sessions)
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
	session controlSession,
	wg *sync.WaitGroup,
	mutex *sync.Mutex,
	active_sessions map[net.Conn]struct{},
) {
	defer wg.Done()
	defer func() {
		session.controlConn.Close()
		mutex.Lock()
		delete(active_sessions, session.controlConn)
		mutex.Unlock()
	}()
	// send server greeting here to kick off control session (clients are expecting it)
	if err := common.SendReply(session.controlConn, constants.ServiceReadyReply); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to send greeting: %v", err)
		return
	}
	// new session always starts with USER then PASS commands
	reader := bufio.NewReader(session.controlConn)
	authenticated := session.authenticateUser(session.controlConn, reader)
	if !authenticated {
		// sessions terminating dont handle errs, send and return
		common.SendReply(session.controlConn, constants.NotLoggedInReply)
		return
	}
	common.SendReply(session.controlConn, constants.UserLoggedInReply)

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
			exit := handleInterpretError(err, session.controlConn)
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
			if err := common.SendReply(session.controlConn, reply); err != nil {
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
func (session *controlSession) PortHandler(args *pi.PortArgs) (pi.FTPReply, error) {
	return pi.FTPReply{}, nil
}

// handle Type command
func (session *controlSession) TypeHandler(args *pi.TypeArgs) (pi.FTPReply, error) {
	return pi.FTPReply{}, nil
}

// handle Mode command
func (session *controlSession) ModeHandler(args *pi.ModeArgs) (pi.FTPReply, error) {
	return pi.FTPReply{}, nil
}

// handle Structure command
func (session *controlSession) StructureHandler(args *pi.StructureArgs) (pi.FTPReply, error) {
	return pi.FTPReply{}, nil
}

// handle Retrieve command
func (session *controlSession) RetrieveHandler(args *pi.RetrArgs) (pi.FTPReply, error) {
	var dataIp string
	var dataPort string
	var done chan bool = make(chan bool, 1) // chan that holds one bool to signal job done
	if session.dataIP == "" {               // use the set dataIP if provided, fallback is client
		dataIp = session.clientIP
	} else {
		dataIp = session.dataIP
	}
	if session.dataPort == "" { // same for port, but fallback to default port
		dataPort = session.clientPort
	} else {
		dataPort = session.dataPort
	}
	serverPort, err := strconv.Atoi(session.serverPort)
	if err != nil {
		return pi.FTPReply{}, fmt.Errorf("Invalid serverPort string: %s", session.serverPort)
	}
	go dataSessionHandler(
		session.serverIp,
		serverPort,
		dataIp,
		dataPort,
		session.dataChan,
		done,
		pi.RETR,
		args.Path,
		session.Type,
		session.Mode,
		session.Structure,
		retrieveHelper,
	)
	// block until a job status returns
	jobStatus := <-session.dataChan
	// failure to start data connection
	if jobStatus.reply.Code != constants.FileStatusOkReply.Code {
		return jobStatus.reply, nil // we act on errors below, just deal with replies for now
	}
	common.SendReply(session.controlConn, jobStatus.reply) // send intermediate reply, transfer is in progress
	// a second reply will either be a success or a failure for some r
	jobStatus = <-session.dataChan
	return jobStatus.reply, nil
}

// handle Store command
func (session *controlSession) StoreHandler(args *pi.StorArgs) (pi.FTPReply, error) {
	var dataIp string
	var dataPort string
	var done chan bool = make(chan bool, 1) // chan that holds one bool to signal job done
	if session.dataIP == "" {               // use the set dataIP if provided, fallback is client
		dataIp = session.clientIP
	} else {
		dataIp = session.dataIP
	}
	if session.dataPort == "" { // same for port, but fallback to default port
		dataPort = session.clientPort
	} else {
		dataPort = session.dataPort
	}
	serverPort, err := strconv.Atoi(session.serverPort)
	if err != nil {
		return pi.FTPReply{}, fmt.Errorf("Invalid serverPort string: %s", session.serverPort)
	}
	go dataSessionHandler(
		session.serverIp,
		serverPort,
		dataIp,
		dataPort,
		session.dataChan,
		done,
		pi.STOR,
		args.Path,
		session.Type,
		session.Mode,
		session.Structure,
		storeHelper,
	)
	// block until a job status returns
	jobStatus := <-session.dataChan
	// failure to start data connection
	if jobStatus.reply.Code != constants.FileStatusOkReply.Code {
		return jobStatus.reply, nil // we act on errors below, just deal with replies for now
	}
	common.SendReply(session.controlConn, jobStatus.reply) // send intermediate reply, transfer is in progress
	// a second reply will either be a success or a failure for some r
	jobStatus = <-session.dataChan
	return jobStatus.reply, nil
}

// handle NOOP command
func (session *controlSession) NOOPHandler() (pi.FTPReply, error) {
	return constants.CommandOkReply, nil
}

// File Operation Helper Functions

func mapFileOpsErrorToFTPReply(err error) (pi.FTPReply, error) {
	var reply pi.FTPReply
	switch {
	case errors.Is(err, constants.ErrFailedDataConnection):
		reply = constants.DataConnectionFailToOpenReply
	case errors.Is(err, constants.ErrDataConnectionClosed):
		reply = constants.DataConnectionAbortedReply
	case errors.Is(err, constants.ErrDataServerIssue):
		reply = constants.DataConnectionServerErrorReply
	case errors.Is(err, constants.ErrFileNotFound):
		reply = constants.FileUnavailableReply
	default:
		return pi.FTPReply{}, fmt.Errorf("dataSessionHandler returned an unregistered error, please add it: %v", err)
	}
	return reply, nil
}

// Create a new thread for handling a data transaction (store/retr)
func dataSessionHandler(
	serverIp string,
	serverPort int,
	destIP string,
	destPort string,
	dataChan chan transferJob,
	doneChan chan bool,
	fileOp pi.FTPTokenType,
	path string,
	fileType pi.TypeCode,
	fileMode pi.ModeCode,
	fileStructure pi.StructureCode,
	dataHandlerFn func(net.Conn, string, pi.TypeCode, pi.ModeCode, pi.StructureCode) error,
) {
	// establish connection
	dialer := net.Dialer{
		Timeout: 5 * time.Second,
		LocalAddr: &net.TCPAddr{
			IP:   net.ParseIP(serverIp),
			Port: serverPort - 1,
		},
	}
	conn, err := dialer.Dial("tcp", destIP+":"+destPort)
	if err != nil { // err connecting to destination
		// send error through chan and return
		dataChan <- transferJob{fileOp, path, constants.DataConnectionFailToOpenReply}
		return
	}
	defer conn.Close()

	// check path is valid before continuing, only for RETR command
	// since STOR creates files if they do not already exist
	if fileOp != pi.STOR {
		_, err = os.Stat(path)
		if errors.Is(err, fs.ErrNotExist) {
			dataChan <- transferJob{fileOp, path, constants.FileUnavailableReply}
			return
		}
	}
	// setup looks good, send intermediate reply before doing the actual transfer
	dataChan <- transferJob{fileOp, path, constants.FileStatusOkReply}
	// run the file transfer helper function passed
	err = dataHandlerFn(conn, path, fileType, fileMode, fileStructure)
	if err != nil { // error occurred handling file
		reply, err := mapFileOpsErrorToFTPReply(err)
		if err != nil { // this error has no registered supply
			reply = pi.MakeFTPReply(999, fmt.Sprintf("Unrecognized error: %v", err))
		}
		dataChan <- transferJob{fileOp, path, reply}
		return
	}
	dataChan <- transferJob{fileOp, path, constants.FileTransferSuccesfulReply} // success
	doneChan <- true
}

func retrieveHelper(
	conn net.Conn,
	path string,
	fileType pi.TypeCode,
	fileMode pi.ModeCode,
	fileStructure pi.StructureCode,
) error {
	common.NotImplementedError()
	return nil
}

func storeHelper(
	conn net.Conn,
	path string,
	fileType pi.TypeCode,
	fileMode pi.ModeCode,
	fileStructure pi.StructureCode,
) error {
	common.NotImplementedError()
	return nil
}
