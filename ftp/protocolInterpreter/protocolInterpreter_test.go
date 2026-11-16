package protocolinterpreter

import (
	"errors"
	"slices"
	"testing"
)

func TestParseCommand(t *testing.T) {
	var testCases = []struct {
		parseFunc   func(string) (FTPCommand, error)
		input       string
		expectedCmd FTPCommand
		caseName    string
	}{
		{parseUser, "GAVIN", MakeUserCmd("GAVIN"), "USER"},
		{parseUser, "Gavin BANANA", MakeUserCmd("Gavin BANANA"), "USER args with space ok"},
		{parsePass, "secret", MakePassCmd("secret"), "PASS"},
		{parsePass, "secret FUSCHIA", MakePassCmd("secret FUSCHIA"), "PASS extra cmd"},
		{parsePass, "secret FUSCHIA ", MakePassCmd("secret FUSCHIA "), "PASS extra cmd with trailing whitespace"},
		{parsePort, "1,1,1,1,0,1", MakePortCmd("1.1.1.1", 0x0001), "PORT lower bound"},
		{parsePort, "1,1,1,1,255,255", MakePortCmd("1.1.1.1", 0xFFFF), "PORT upper bound"},
		{parseType, "A N", MakeTypeCmd('A', 'N', 0), "TYPE A N"},
		{parseType, "A T", MakeTypeCmd('A', 'T', 0), "TYPE A T"},
		{parseType, "A C", MakeTypeCmd('A', 'C', 0), "TYPE A C"},
		{parseType, "E N", MakeTypeCmd('E', 'N', 0), "TYPE E N"},
		{parseType, "E T", MakeTypeCmd('E', 'T', 0), "TYPE E T"},
		{parseType, "E C", MakeTypeCmd('E', 'C', 0), "TYPE E C"},
		{parseType, "I", MakeTypeCmd('I', NONE, 0), "TYPE I"},
		{parseType, "L 1", MakeTypeCmd('L', NONE, 1), "TYPE L lower bound"},
		{parseType, "L 255", MakeTypeCmd('L', NONE, 255), "TYPE L upper bound"},
		{parseType, "a n", MakeTypeCmd('A', 'N', 0), "TYPE a n"},
		{parseType, "a t", MakeTypeCmd('A', 'T', 0), "TYPE a t"},
		{parseType, "a c", MakeTypeCmd('A', 'C', 0), "TYPE a c"},
		{parseType, "e n", MakeTypeCmd('E', 'N', 0), "TYPE e n"},
		{parseType, "e t", MakeTypeCmd('E', 'T', 0), "TYPE e t"},
		{parseType, "e c", MakeTypeCmd('E', 'C', 0), "TYPE e c"},
		{parseType, "i", MakeTypeCmd('I', NONE, 0), "TYPE i"},
		{parseType, "l 1", MakeTypeCmd('L', NONE, 1), "TYPE l lower bound"},
		{parseType, "l 255", MakeTypeCmd('L', NONE, 255), "TYPE l upper bound"},
		{parseType, "A", MakeTypeCmd('A', 'N', 0), "TYPE A missing arg"},
		{parseType, "E", MakeTypeCmd('E', 'N', 0), "TYPE E missing arg"},
		{parseMode, "S", MakeModeCmd('S'), "MODE S"},
		{parseMode, "B", MakeModeCmd('B'), "MODE B"},
		{parseMode, "C", MakeModeCmd('C'), "MODE C"},
		{parseMode, "s", MakeModeCmd('S'), "MODE s"},
		{parseMode, "b", MakeModeCmd('B'), "MODE b"},
		{parseMode, "c", MakeModeCmd('C'), "MODE c"},
		{parseStru, "F", MakeStruCmd('F'), "STRU F"},
		{parseStru, "R", MakeStruCmd('R'), "STRU R"},
		{parseStru, "P", MakeStruCmd('P'), "STRU P"},
		{parseStru, "f", MakeStruCmd('F'), "STRU f"},
		{parseStru, "r", MakeStruCmd('R'), "STRU r"},
		{parseStru, "p", MakeStruCmd('P'), "STRU p"},
		{parseRetr, "~/Downloads", MakeRetrCmd("~/Downloads"), "RETR"},
		{parseRetr, "~/Downloads FAIL?", MakeRetrCmd("~/Downloads FAIL?"), "RETR arg with space"},
		{parseStor, "~/Downloads", MakeStorCmd("~/Downloads"), "STOR"},
		{parseStor, "~/Downloads FAIL?", MakeStorCmd("~/Downloads FAIL?"), "STOR arg with space"},
	}

	for _, tc := range testCases {
		actualCmd, err := tc.parseFunc(tc.input)
		if err != nil { // call failed, raise err
			t.Fatalf("%s case: %v", tc.caseName, err.Error())
		}
		if actualCmd != tc.expectedCmd { // expected command state wron
			t.Fatalf("%s case: found cmd %#v, expected cmd %#v", tc.caseName, actualCmd, tc.expectedCmd)
		}
	}
}

func TestInterpretErrorClassification(t *testing.T) {
	var testCases = []struct {
		input string
		err   error
	}{
		{"UNKNOWN\r\n", ErrInvalidCommand},
		{"USERGAVIN\r\n", ErrInvalidCommand},
		{"\r\n", ErrInvalidCommand},
		{"NOOP\n", ErrInvalidCommand},
		{"", ErrInvalidCommand},
		{"TYPE X\r\n", ErrInvalidArguments},
		{"TYPE A X\r\n", ErrInvalidArguments},
		{"TYPE I extra\r\n", ErrInvalidArguments},
		{"TYPE L 0\r\n", ErrInvalidArguments},
		{"MODE X\r\n", ErrInvalidArguments},
		{"STRU X\r\n", ErrInvalidArguments},
		{"PORT 1,2,3\r\n", ErrInvalidArguments},
		{"PORT 256,0,0,1,1,1\r\n", ErrInvalidArguments},
		{"PORT 127,0,0,1,256,1\r\n", ErrInvalidArguments},
		{"PORT ::ffff:192,0,2,1,1,1\r\n", ErrInvalidArguments},
		{"USER\r\n", ErrInvalidArguments},
		{"PASS\r\n", ErrInvalidArguments},
		{"PORT\r\n", ErrInvalidArguments},
		{"TYPE\r\n", ErrInvalidArguments},
		{"MODE\r\n", ErrInvalidArguments},
		{"STRU\r\n", ErrInvalidArguments},
		{"RETR\r\n", ErrInvalidArguments},
		{"STOR\r\n", ErrInvalidArguments},
		{"NOOP extra\r\n", ErrInvalidArguments},
		{"QUIT extra\r\n", ErrInvalidArguments},
		{"NOOP \r\n", ErrInvalidArguments},
		{"QUIT \r\n", ErrInvalidArguments},
	}
	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			cmds, err := Interpret(tc.input)
			if !errors.Is(err, tc.err) {
				t.Fatalf("expected error %v, found %v", tc.err, err)
			}
			if len(cmds) != 0 {
				t.Fatalf("invalid command returned commands: %#v", cmds)
			}
		})
	}
}

func TestParseArgsUnexpectedCommand(t *testing.T) {
	_, err := parseArgs(FTPTokenType(999), "")
	if err == nil || errors.Is(err, ErrInvalidCommand) || errors.Is(err, ErrInvalidArguments) {
		t.Fatalf("expected an internal error, found %v", err)
	}
}

func TestParseCommandExceptionCases(t *testing.T) {
	var testCases = []struct {
		parseFunc func(string) (FTPCommand, error)
		input     string
		caseName  string
	}{
		{parsePort, "", "PORT empty"},
		{parsePort, "1,1,1,1,0,0", "PORT too low"},
		{parsePort, "1,1,1,1,256,256", "PORT too high"},
		{parsePort, "256,1,1,1,1,1", "PORT fail uint8 parse"},
		{parseType, "", "TYPE empty"},
		{parseType, "WALNUT", "TYPE unrecognized type"},
		{parseType, "A Z", "TYPE A unrecognized format"},
		{parseType, "A N extra", "TYPE A extra arg"},
		{parseType, "E D", "TYPE E unrecognized format"},
		{parseType, "I extra", "TYPE extra arg"},
		{parseType, "L", "TYPE L missing arg"},
		{parseType, "L 255 extra", "TYPE L extra arg"},
		{parseType, "L 0", "TYPE L too low"},
		{parseType, "L 256", "TYPE L too high"},
		{parseType, "L WORD", "TYPE L fail uint8 parse"},
		{parseMode, "", "MODE empty"},
		{parseMode, "R", "MODE unrecognized mode"},
		{parseStru, "W", "STRU unrecognized mode"},
	}

	for _, tc := range testCases {
		actual, err := tc.parseFunc(tc.input)
		if err == nil {
			t.Fatalf("%s case: expected failure, found %#v", tc.caseName, actual)
		}
	}
}

func TestInterpret(t *testing.T) {
	var testCases = []struct {
		input        string
		expectedCmds []FTPCommand
		caseName     string
	}{
		// Individual command parsing cases
		{"USER GAVIN\r\n", []FTPCommand{MakeUserCmd("GAVIN")}, "USER basic"},
		{"PASS GAVIN\r\n", []FTPCommand{MakePassCmd("GAVIN")}, "PASS basic"},
		{"QUIT\r\n", []FTPCommand{MakeQuitCmd()}, "QUIT basic"},
		{"PORT 1,1,1,1,000,123\r\n", []FTPCommand{MakePortCmd("1.1.1.1", 123)}, "PORT basic"},
		{"TYPE A N\r\n", []FTPCommand{MakeTypeCmd('A', 'N', 0)}, "TYPE A N basic"},
		{"TYPE A T\r\n", []FTPCommand{MakeTypeCmd('A', 'T', 0)}, "TYPE A T basic"},
		{"TYPE A C\r\n", []FTPCommand{MakeTypeCmd('A', 'C', 0)}, "TYPE A C basic"},
		{"TYPE E N\r\n", []FTPCommand{MakeTypeCmd('E', 'N', 0)}, "TYPE E N basic"},
		{"TYPE E T\r\n", []FTPCommand{MakeTypeCmd('E', 'T', 0)}, "TYPE E T basic"},
		{"TYPE E C\r\n", []FTPCommand{MakeTypeCmd('E', 'C', 0)}, "TYPE E C basic"},
		{"TYPE I\r\n", []FTPCommand{MakeTypeCmd('I', NONE, 0)}, "TYPE I basic"},
		{"TYPE L 123\r\n", []FTPCommand{MakeTypeCmd('L', NONE, 123)}, "TYPE L basic"},
		{"MODE S\r\n", []FTPCommand{MakeModeCmd('S')}, "MODE S basic"},
		{"MODE B\r\n", []FTPCommand{MakeModeCmd('B')}, "MODE B basic"},
		{"MODE C\r\n", []FTPCommand{MakeModeCmd('C')}, "MODE C basic"},
		{"STRU F\r\n", []FTPCommand{MakeStruCmd('F')}, "STRU F basic"},
		{"STRU R\r\n", []FTPCommand{MakeStruCmd('R')}, "STRU R basic"},
		{"STRU P\r\n", []FTPCommand{MakeStruCmd('P')}, "STRU P basic"},
		{"RETR ~/Downloads/cool_file\r\n", []FTPCommand{MakeRetrCmd("~/Downloads/cool_file")}, "RETR basic"},
		{"STOR ~/Downloads/cool_file\r\n", []FTPCommand{MakeStorCmd("~/Downloads/cool_file")}, "STOR basic"},
		{"UsEr GAVIN\r\n", []FTPCommand{MakeUserCmd("GAVIN")}, "Normalize command case"},
		{"NOOP\r\n", []FTPCommand{MakeNOOPCmd()}, "NOOP basic"},

		// More complicated strings to parse
		{
			"USER GAVIN\r\nPASS admin\r\n",
			[]FTPCommand{MakeUserCmd("GAVIN"), MakePassCmd("admin")},
			"complex1",
		},
		{
			"TYPE L 255\r\nSTRU R\r\nSTOR /dev/null\r\nUSER GANDALF\r\n",
			[]FTPCommand{MakeTypeCmd('L', NONE, 255), MakeStruCmd('R'), MakeStorCmd("/dev/null"), MakeUserCmd("GANDALF")},
			"complex2",
		},
	}

	for _, tc := range testCases {
		actual, err := Interpret(tc.input)
		if err != nil { // parsing failed!
			t.Fatalf("%s case: %v", tc.caseName, err.Error())
		}
		if !slices.Equal(actual, tc.expectedCmds) {
			t.Fatalf("%s case: found cmds %#v, expected cmds %#v", tc.caseName, actual, tc.expectedCmds)
		}
	}
}

func TestInterpretExceptionCases(t *testing.T) {
	var testCases = []struct {
		input    string
		caseName string
	}{
		{"USERGAVIN\r\n", "Missing space"},
		{"USER\r\n", "Missing args"},
		{"USER GAVIN", "Missing <CRLF>"},
		{"", "Empty input"},
		{"TYPE MODE S\r\n", "Missing arg"},
		{"PORT ::ffff:192,0,2,1,1,1\r\n", "sneaky IPv6 injection"},
	}

	for _, tc := range testCases {
		actual, err := Interpret(tc.input)
		if err == nil { // why didnt it fail???
			t.Fatalf("%s case: expected failure, found %v", tc.caseName, actual)
		}
	}
}

// Happy Path Test for Stringer Methods
// (assume always valid as parsing handles validation)
func TestFTPCommandStringInterface(t *testing.T) {
	var testCases = []struct {
		input    FTPCommand
		expected string
		caseName string
	}{
		{MakeUserCmd("bubba"), "USER bubba", "basic user"},
		{MakePassCmd("secret"), "PASS secret", "basic pass"},
		{MakePortCmd("192.168.1.2", 0x08ff), "PORT 192,168,1,2,8,255", "basic port"},
		{MakeRetrCmd("~/Downloads"), "RETR ~/Downloads", "basic retr"},
		{MakeStorCmd("~/Downloads"), "STOR ~/Downloads", "basic stor"},
		{MakeTypeCmd(ASCII, NONPRINT, 0), "TYPE A N", "basic Type A"},
		{MakeTypeCmd(EBCDIC, TELNET, 0), "TYPE E T", "basic Type E"},
		{MakeTypeCmd('I', NONE, 0), "TYPE I", "basic Type I"},
		{MakeTypeCmd(LOCAL, NONE, 9), "TYPE L 9", "basic Type L"},
		{MakeModeCmd(STREAM), "MODE S", "basic Mode S"},
		{MakeStruCmd(FILE), "STRU F", "basic STRU F"},
	}

	for _, tc := range testCases {
		actual := tc.input.String()
		if actual != tc.expected {
			t.Fatalf("%s case: found string %s, expected %s", tc.caseName, actual, tc.expected)
		}
	}
}
