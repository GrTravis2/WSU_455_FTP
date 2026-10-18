// WSU 455 Semester Project - FTP
// Functions related to interpreting
// FTP messages into FTP commands
package protocolinterpreter

import (
	"fmt"
	"log"
	"strconv"
	"strings"
)

// FTP Command Enum Type
type FTPTokenType uint

const (
	/* COMMAND TOKENS */

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

	/* ARGUMENT TOKENS */

	STRING        // NVT ASCII STRING
	NUMBER        // DECIMAL INT [1, 255]
	FORMCODE      // N | T | C
	TYPECODE      // A | E | I | L
	STRUCTURECODE // F | R | P
	MODECODE      // S | B | C
)

type FTPCommand struct {
	Token FTPTokenType
	Args  any
}

/* Standard Type Args */

type UserArgs struct{ Username string }
type PassArgs struct{ Pasword string }
type PortArgs struct{ PortNum uint8 }
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

type ModeCode byte

const (
	STREAM     ModeCode = 'S'
	BLOCK      ModeCode = 'B'
	COMPRESSED ModeCode = 'C'
)

type FormatCode byte

const (
	NONPRINT FormatCode = 'N'
	TELNET   FormatCode = 'T'
	CARRIAGE FormatCode = 'C'
	NONE     FormatCode = '\x00' // IMAGE has not format type, use NULL
)

type StructureCode byte

const (
	FILE   StructureCode = 'F'
	RECORD StructureCode = 'R'
	PAGE   StructureCode = 'P'
)

type TypeArgs struct {
	Code     TypeCode
	Format   FormatCode
	ByteSize uint8
}
type ModeArgs struct{ Code ModeCode }
type FormArgs struct{ Code FormatCode }
type StructureArgs struct{ Code StructureCode }

var cmdMapping = map[string]FTPCommand{
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
	words := strings.Split(msg, " ")
	cmds, err := parse(words)
	if err != nil {
		return []FTPCommand{}, err
	}

	return cmds, nil
}

// Convert a string slice into a slice of tokens to be parsed
func parse(words []string) ([]FTPCommand, error) {
	tokens := make([]FTPCommand, 0, 64) // FTPToken buffer
	return parseLoop(words, tokens)     // pass buffer as reference slice
}

// Recursive helper for lexing word input
func parseLoop(words []string, tokens []FTPCommand) ([]FTPCommand, error) {
	if len(words) == 0 { // base case
		return tokens, nil
	}
	// parse another command
	w := strings.ToUpper(words[0])
	cmd, isCmd := cmdMapping[w]
	if !isCmd { // lex should always start w/ a command!
		return tokens,
			fmt.Errorf(
				"expected an FTP command, found argument '%s'",
				w,
			)
	}
	updated_words, err := popArgs(&cmd, words[1:])
	if err != nil {
		return tokens, err
	}
	return parseLoop(updated_words, append(tokens, cmd))
}

func popArgs(cmd *FTPCommand, words []string) ([]string, error) {
	switch cmd.Token {
	case USER:
		return parseUser(cmd, words)
	case PASS:
		return parsePass(cmd, words)
	case PORT:
		return parsePort(cmd, words)
	case TYPE:
		return parseType(cmd, words)
	case MODE:
		return parseMode(cmd, words)
	case STRU:
		return parseStru(cmd, words)
	case RETR:
		return parseRetr(cmd, words)
	case STOR:
		return parseStor(cmd, words)
	case QUIT, NOOP: // no args commands, do nothing return unchanged slice
		return words, nil
	default:
		// we check earlier that command is valid, if we get here
		// panic and report so we can see how this could have happened!
		log.Panicf("unexpected command found '%v'", cmd.Token)
		return []string{}, nil // cannot get here, but makes compiler happy
	}
}

func parseUser(cmd *FTPCommand, words []string) ([]string, error) {
	if len(words) == 0 {
		return words, missingArgumentError(cmd.Token)
	}
	args, _ := cmd.Args.(UserArgs)
	args.Username = words[0]
	cmd.Args = args
	return words[1:], nil
}

func parsePass(cmd *FTPCommand, words []string) ([]string, error) {
	if len(words) == 0 {
		return words, missingArgumentError(cmd.Token)
	}
	args, _ := cmd.Args.(PassArgs)
	args.Pasword = words[0]
	cmd.Args = args
	return words[1:], nil
}

func parsePort(cmd *FTPCommand, words []string) ([]string, error) {
	if len(words) == 0 {
		return words, missingArgumentError(cmd.Token)
	}
	args, _ := cmd.Args.(PortArgs)
	portNum, err := strconv.ParseUint(words[0], 10, 8)
	if err != nil {
		return words, fmt.Errorf("unable to parse '%s' into uint8", words[0])
	}
	args.PortNum = uint8(portNum)
	cmd.Args = args
	return words[1:], nil
}

func parseType(cmd *FTPCommand, words []string) ([]string, error) {
	if len(words) == 0 {
		return words, missingArgumentError(cmd.Token)
	}
	args, _ := cmd.Args.(TypeArgs)
	switch words[0] {
	case "A":
		args.Code = ASCII
	case "E":
		args.Code = EBCDIC
	case "I":
		args.Code = IMAGE
	case "L":
		args.Code = LOCAL
	default:
		return words, fmt.Errorf("expected type code, found '%s'", words[0])
	}
	switch args.Code {
	case IMAGE:
		args.Format = NONE
		cmd.Args = args
		return words[1:], nil
	case LOCAL:
		if len(words) < 2 {
			formatString := "missing required byte size args for '%v' command"
			return words, fmt.Errorf(formatString, cmd.Token)
		}
		args.Format = NONE
		byteSize, err := strconv.ParseUint(words[1], 10, 8)
		if err != nil {
			formatString := "unable to parse byte size '%v' into uint8"
			return words, fmt.Errorf(formatString, words[1])
		}
		args.ByteSize = uint8(byteSize)
		return words[2:], nil
	default:
		if len(words) < 2 {
			formatStr := "missing required format code arg for '%v' command"
			return words, fmt.Errorf(formatStr, cmd.Token)
		}
		code := words[1]
		switch code {
		case "N":
			args.Format = NONPRINT
		case "T":
			args.Format = TELNET
		case "C":
			args.Format = CARRIAGE
		default:
			return words, fmt.Errorf("expected type code, found '%s'", code)
		}
		cmd.Args = args
		return words[2:], nil
	}
}

func parseMode(cmd *FTPCommand, words []string) ([]string, error) {
	if len(words) == 0 {
		return words, missingArgumentError(cmd.Token)
	}
	args, _ := cmd.Args.(ModeArgs)
	switch words[0] {
	case "S":
		args.Code = STREAM
	case "B":
		args.Code = BLOCK
	case "C":
		args.Code = COMPRESSED
	default:
		return words, fmt.Errorf("exected mode code, found '%s'", words[0])
	}
	cmd.Args = args
	return words[1:], nil
}

func parseStru(cmd *FTPCommand, words []string) ([]string, error) {
	if len(words) == 0 {
		return words, missingArgumentError(cmd.Token)
	}
	args, _ := cmd.Args.(StructureArgs)
	switch words[0] {
	case "F":
		args.Code = FILE
	case "R":
		args.Code = RECORD
	case "P":
		args.Code = PAGE
	default:
		return words, fmt.Errorf("exected mode code, found '%s'", words[0])
	}
	cmd.Args = args
	return words[1:], nil
}

func parseRetr(cmd *FTPCommand, words []string) ([]string, error) {
	if len(words) == 0 {
		return words, missingArgumentError(cmd.Token)
	}
	args, _ := cmd.Args.(RetrArgs)
	args.Path = words[0]
	cmd.Args = args
	return words[1:], nil
}

func parseStor(cmd *FTPCommand, words []string) ([]string, error) {
	if len(words) == 0 {
		return words, missingArgumentError(cmd.Token)
	}
	args, _ := cmd.Args.(StorArgs)
	args.Path = words[0]
	cmd.Args = args
	return words[1:], nil
}

func missingArgumentError(token FTPTokenType) error {
	return fmt.Errorf("command '%v' missing required arg", token)
}
