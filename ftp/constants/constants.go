// WSU 455 Semester Project - FTP
// FTP common constants
package constants

import (
	"errors"
	pi "wsu455ftp/ftp/protocolInterpreter"
)

// Server constants
const DEFAULT_CONTROL_PORT string = "21"
const DEFAULT_DATA_PORT string = "20"
const DEFAULT_USER string = "user"
const DEFAULT_PASS string = "password"

/* Constant Replies */
var ServiceReadyReply = pi.MakeFTPReply(220, "Service ready")
var UsernameOkReply = pi.MakeFTPReply(331, "User name ok, need password")
var UserLoggedInReply = pi.MakeFTPReply(230, "User logged in")
var NotLoggedInReply = pi.MakeFTPReply(530, "Not logged in")
var QuitConnectionReply = pi.MakeFTPReply(221, "Service closing control connection")
var CommandOkReply = pi.MakeFTPReply(200, "Command okay")
var DataConnectionFailToOpenReply = pi.MakeFTPReply(425, "Can't open data connection")
var DataConnectionAbortedReply = pi.MakeFTPReply(426, "Connection closed; transfer aborted")
var DataConnectionServerErrorReply = pi.MakeFTPReply(451, "Requested action aborted due to local server error")
var FileUnavailableReply = pi.MakeFTPReply(550, "File unavailable")
var FileStatusOkReply = pi.MakeFTPReply(150, "File status ok; about to open data connection")
var FileTransferSuccessfulReply = pi.MakeFTPReply(226, "Closing data connection, file transfer successful")

/* Constant Errors */
var ErrFailedDataConnection = errors.New("Unable to establish data connection")
var ErrDataConnectionClosed = errors.New("Data connection closed unexpectedly")
var ErrDataServerIssue = errors.New("Data transfer failed due to server error")
var ErrFileNotFound = errors.New("Data transfer failed due file not found")
