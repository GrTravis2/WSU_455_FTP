// WSU 455 Semester Project - FTP
// FTP common constants
package constants

import (
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
var InvalidArgumentsReply = pi.MakeFTPReply(501, "Syntax error in parameters or arguments")
var ParameterNotImplementedReply = pi.MakeFTPReply(504, "Command not implemented for that parameter")

// FTP Errors
