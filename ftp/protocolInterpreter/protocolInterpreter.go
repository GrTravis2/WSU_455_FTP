// WSU 455 Semester Project - FTP
// Functions related to interpreting
// FTP messages into FTP commands
package protocolinterpreter

import (
	"fmt"
	"log"
	"net/netip"
	"strconv"
	"strings"
)

// FTP Command Enum Type
type FTPTokenType uint

/* COMMAND TOKENS */

const (
	USER FTPTokenType = iota // USER <username ::= string>
	PASS                     // PASS <password ::= string>
	QUIT                     // QUIT
	PORT                     // PORT <port ::= number>
	TYPE                     // TYPE <type-code ::= A <form-code> | E <form-code> | I | L <byte-size>>
	MODE                     // MODE <mode-code ::= S | B | C>
	STRU                     // STRU <structure-code ::= F | R | P>
	RETR                     // RETR <path-name ::= string>
	STOR                     //	STOR <path-name ::= string>
	NOOP                     // NOOP
)

// Implement Stringer interface for enum pretty printing
func (t FTPTokenType) String() string {
	switch t {
	case USER:
		return "USER"
	case PASS:
		return "PASS"
	case QUIT:
		return "QUIT"
	case PORT:
		return "PORT"
	case TYPE:
		return "TYPE"
	case MODE:
		return "MODE"
	case STRU:
		return "STRU"
	case RETR:
		return "RETR"
	case STOR:
		return "STOR"
	case NOOP:
		return "NOOP"
	default:
		log.Panicf("missing Stringer for token %v, please add it!", uint(t))
		return ""
	}
}

type FTPCommand struct {
	Token FTPTokenType
	Args  any
}

/* Standard Type Args */

type UserArgs struct{ Username string }
type PassArgs struct{ Pasword string }
type PortArgs struct {
	Ip   netip.Addr
	Port uint16
}
type RetrArgs struct{ Path string }
type StorArgs struct{ Path string }

/* Code Args - They have finite set of chars */

type TypeCode byte

const (
	ASCII  TypeCode = 'A'
	EBCDIC TypeCode = 'E'
	IMAGE  TypeCode = 'I'
	LOCAL  TypeCode = 'L'
)

type TypeArgs struct {
	Code     TypeCode
	Format   FormatCode
	ByteSize uint8 // logical byte size, only relevant if Format is LOCAL
}

type ModeCode byte

const (
	STREAM     ModeCode = 'S'
	BLOCK      ModeCode = 'B'
	COMPRESSED ModeCode = 'C'
)

type ModeArgs struct{ Code ModeCode }

type FormatCode byte

const (
	NONPRINT FormatCode = 'N'
	TELNET   FormatCode = 'T'
	CARRIAGE FormatCode = 'C'
	NONE     FormatCode = '\x00' // IMAGE has not format type, use NULL
)

type FormArgs struct{ Code FormatCode }

type StructureCode byte

const (
	FILE   StructureCode = 'F'
	RECORD StructureCode = 'R'
	PAGE   StructureCode = 'P'
)

type StructureArgs struct{ Code StructureCode }

var CMDMAPPING = map[string]FTPCommand{
	"USER": {USER, UserArgs{}},
	"PASS": {PASS, PassArgs{}},
	"QUIT": {QUIT, nil},
	"PORT": {PORT, PortArgs{}},
	"TYPE": {TYPE, TypeArgs{}},
	"MODE": {MODE, ModeArgs{}},
	"STRU": {STRU, StructureArgs{}},
	"RETR": {RETR, RetrArgs{}},
	"STOR": {STOR, StorArgs{}},
	"NOOP": {NOOP, nil},
}

// parse an FTP control message into FTP Commands
func Interpret(msg string) ([]FTPCommand, error) {
	words := strings.Split(strings.TrimSuffix(msg, "\r\n"), "\r\n")
	cmds, err := parse(words)
	if err != nil {
		return []FTPCommand{}, err
	}

	return cmds, nil
}

// Convert a string slice into a slice of commands for server/client to action
func parse(words []string) ([]FTPCommand, error) {
	cmds := make([]FTPCommand, 0, 64) // FTPCommand buffer
	for _, word := range words {
		// split word into cmd type and args
		token, args, _ := strings.Cut(word, " ")

		// match cmd type to command object
		t := strings.ToUpper(token)
		cmd, isCmd := CMDMAPPING[t]
		if !isCmd { // parse should always start w/ a command!
			return cmds,
				fmt.Errorf(
					"expected an FTP command, found '%s'",
					t,
				)
		}
		// if parsing a command with args
		var parsedCmd FTPCommand
		var err error
		if cmd.Token != QUIT && cmd.Token != NOOP {
			if len(args) == 0 {
				return []FTPCommand{}, missingRequiredArgError(cmd.Token, args)
			}
			// then call parsing function to get parsed command obj
			parsedCmd, err = parseArgs(cmd.Token, args)
			if err != nil {
				fmtString := "unable to parse command args '%s': %v"
				return []FTPCommand{}, fmt.Errorf(fmtString, args, err.Error())
			}
		} else { // command w/ no args to parse
			parsedCmd = FTPCommand{cmd.Token, nil}
		}
		cmds = append(cmds, parsedCmd)
	}

	return cmds, nil
}

func parseArgs(token FTPTokenType, args string) (FTPCommand, error) {
	switch token {
	case USER:
		return parseUser(args)
	case PASS:
		return parsePass(args)
	case PORT:
		return parsePort(args)
	case TYPE:
		return parseType(args)
	case MODE:
		return parseMode(args)
	case STRU:
		return parseStru(args)
	case RETR:
		return parseRetr(args)
	case STOR:
		return parseStor(args)
	default:
		// we check earlier that command is valid, if we get here
		// panic and report so we can see how this could have happened!
		panic(fmt.Sprintf("unexpected command found '%v'", token))
	}
}

func parseUser(args string) (FTPCommand, error) {
	return FTPCommand{USER, UserArgs{args}}, nil
}

func parsePass(args string) (FTPCommand, error) {
	return FTPCommand{PASS, PassArgs{args}}, nil
}

func parsePort(args string) (FTPCommand, error) {
	fields := strings.Split(args, ",")
	if len(fields) != 6 {
		return FTPCommand{}, missingRequiredArgError(PORT, args)
	}
	addr := strings.Join(fields[:4], ".")
	ip, err := netip.ParseAddr(addr)
	if err != nil || !ip.Is4() {
		s := "unable to parse '%s' into IPv4: %v"
		return FTPCommand{}, fmt.Errorf(s, addr, err.Error())
	}
	var uints [2]uint8
	for i, s := range fields[4:] {
		num, err := strconv.ParseUint(s, 10, 8)
		if err != nil {
			fmtString := "unable to parse '%s' into uint8"
			return FTPCommand{}, fmt.Errorf(fmtString, s)
		}
		uints[i] = uint8(num)
	}
	port := (uint16(uints[0]) << 8) + uint16(uints[1])
	if port == 0 {
		return FTPCommand{}, fmt.Errorf("cannot have port number %d", port)
	}
	return FTPCommand{PORT, PortArgs{ip, port}}, nil
}

func parseType(args string) (FTPCommand, error) {
	chars := strings.Split(args, " ")

	var code TypeCode
	var format FormatCode
	var byteSize uint8 = 0
	switch strings.ToUpper(chars[0]) {
	case "A":
		code = ASCII
	case "E":
		code = EBCDIC
	case "I":
		code = IMAGE
	case "L":
		code = LOCAL
	default:
		return FTPCommand{}, fmt.Errorf("expected type code, found '%s'", chars[0])
	}
	switch code {
	case IMAGE:
		format = NONE
		byteSize = 0
	case LOCAL:
		if len(chars) < 2 {
			formatString := "missing required byte size args for '%v' command"
			return FTPCommand{}, fmt.Errorf(formatString, LOCAL)
		}
		format = NONE
		size, err := strconv.ParseUint(chars[1], 10, 8)
		if err != nil {
			formatString := "unable to parse byte size '%v' into uint8"
			return FTPCommand{}, fmt.Errorf(formatString, chars[1])
		}
		if size == 0 {
			return FTPCommand{}, fmt.Errorf("cannot have byte size of %d", size)
		}
		byteSize = uint8(size)
	case ASCII, EBCDIC:
		if len(chars) == 1 { // no format code, default is NONPRINT
			format = NONPRINT
		} else {
			switch strings.ToUpper(chars[1]) {
			case "N":
				format = NONPRINT
			case "T":
				format = TELNET
			case "C":
				format = CARRIAGE
			default:
				return FTPCommand{}, fmt.Errorf("expected format code, found '%v'", code)
			}
		}
	}
	return FTPCommand{TYPE, TypeArgs{code, format, byteSize}}, nil
}

func parseMode(args string) (FTPCommand, error) {
	var code ModeCode
	switch strings.ToUpper(args) {
	case "S":
		code = STREAM
	case "B":
		code = BLOCK
	case "C":
		code = COMPRESSED
	default:
		return FTPCommand{}, fmt.Errorf("exected mode code, found '%s'", args)
	}
	return FTPCommand{MODE, ModeArgs{code}}, nil
}

func parseStru(args string) (FTPCommand, error) {
	var code StructureCode
	switch strings.ToUpper(args) {
	case "F":
		code = FILE
	case "R":
		code = RECORD
	case "P":
		code = PAGE
	default:
		return FTPCommand{}, fmt.Errorf("exected mode code, found '%s'", args)
	}
	return FTPCommand{STRU, StructureArgs{code}}, nil
}

func parseRetr(args string) (FTPCommand, error) {
	return FTPCommand{RETR, RetrArgs{args}}, nil
}

func parseStor(args string) (FTPCommand, error) {
	return FTPCommand{STOR, StorArgs{args}}, nil
}

func missingRequiredArgError(token FTPTokenType, args string) error {
	fmtString := "command '%v' missing required arg, found args '%s'"
	return fmt.Errorf(fmtString, token, args)
}

/* Private helpers for creating FTPCommands, it will help keep things more concise for testing */

func makeUserCmd(user string) FTPCommand { return FTPCommand{USER, UserArgs{user}} }
func makePassCmd(pass string) FTPCommand { return FTPCommand{PASS, PassArgs{pass}} }
func makeQuitCmd() FTPCommand            { return FTPCommand{QUIT, nil} }
func makePortCmd(ip string, port uint16) FTPCommand {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		log.Panicf("unable to parse ip addr '%s' into IPv4", ip)
	}
	return FTPCommand{PORT, PortArgs{addr, port}}
}
func makeTypeCmd(typeCode TypeCode, formatCode FormatCode, byteSize uint8) FTPCommand {
	return FTPCommand{TYPE, TypeArgs{typeCode, formatCode, byteSize}}
}
func makeModeCmd(code ModeCode) FTPCommand      { return FTPCommand{MODE, ModeArgs{code}} }
func makeStruCmd(code StructureCode) FTPCommand { return FTPCommand{STRU, StructureArgs{code}} }
func makeRetrCmd(path string) FTPCommand        { return FTPCommand{RETR, RetrArgs{path}} }
func makeStorCmd(path string) FTPCommand        { return FTPCommand{STOR, StorArgs{path}} }
func makeNOOPCmd() FTPCommand                   { return FTPCommand{NOOP, nil} }
