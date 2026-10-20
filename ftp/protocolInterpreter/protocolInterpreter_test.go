package protocolinterpreter

import (
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
		{parseUser, "GAVIN", makeUserCmd("GAVIN"), "USER"},
		{parseUser, "Gavin BANANA", makeUserCmd("Gavin BANANA"), "USER args with space ok"},
		{parsePass, "secret", makePassCmd("secret"), "PASS"},
		{parsePass, "secret FUSCHIA", makePassCmd("secret FUSCHIA"), "PASS extra cmd"},
		{parsePort, "1,1,1,1,0,1", makePortCmd("1.1.1.1", 0x0001), "PORT lower bound"},
		{parsePort, "1,1,1,1,255,255", makePortCmd("1.1.1.1", 0xFFFF), "PORT upper bound"},
		{parseType, "A N", makeTypeCmd('A', 'N', 0), "TYPE A N"},
		{parseType, "A T", makeTypeCmd('A', 'T', 0), "TYPE A T"},
		{parseType, "A C", makeTypeCmd('A', 'C', 0), "TYPE A C"},
		{parseType, "E N", makeTypeCmd('E', 'N', 0), "TYPE E N"},
		{parseType, "E T", makeTypeCmd('E', 'T', 0), "TYPE E T"},
		{parseType, "E C", makeTypeCmd('E', 'C', 0), "TYPE E C"},
		{parseType, "I", makeTypeCmd('I', NONE, 0), "TYPE I"},
		{parseType, "L 1", makeTypeCmd('L', NONE, 1), "TYPE L lower bound"},
		{parseType, "L 255", makeTypeCmd('L', NONE, 255), "TYPE L upper bound"},
		{parseType, "a n", makeTypeCmd('A', 'N', 0), "TYPE a n"},
		{parseType, "a t", makeTypeCmd('A', 'T', 0), "TYPE a t"},
		{parseType, "a c", makeTypeCmd('A', 'C', 0), "TYPE a c"},
		{parseType, "e n", makeTypeCmd('E', 'N', 0), "TYPE e n"},
		{parseType, "e t", makeTypeCmd('E', 'T', 0), "TYPE e t"},
		{parseType, "e c", makeTypeCmd('E', 'C', 0), "TYPE e c"},
		{parseType, "i", makeTypeCmd('I', NONE, 0), "TYPE i"},
		{parseType, "l 1", makeTypeCmd('L', NONE, 1), "TYPE l lower bound"},
		{parseType, "l 255", makeTypeCmd('L', NONE, 255), "TYPE l upper bound"},
		{parseType, "A", makeTypeCmd('A', 'N', 0), "TYPE A missing arg"},
		{parseType, "E", makeTypeCmd('E', 'N', 0), "TYPE E missing arg"},
		{parseMode, "S", makeModeCmd('S'), "MODE S"},
		{parseMode, "B", makeModeCmd('B'), "MODE B"},
		{parseMode, "C", makeModeCmd('C'), "MODE C"},
		{parseMode, "s", makeModeCmd('S'), "MODE s"},
		{parseMode, "b", makeModeCmd('B'), "MODE b"},
		{parseMode, "c", makeModeCmd('C'), "MODE c"},
		{parseStru, "F", makeStruCmd('F'), "STRU F"},
		{parseStru, "R", makeStruCmd('R'), "STRU R"},
		{parseStru, "P", makeStruCmd('P'), "STRU P"},
		{parseStru, "f", makeStruCmd('F'), "STRU f"},
		{parseStru, "r", makeStruCmd('R'), "STRU r"},
		{parseStru, "p", makeStruCmd('P'), "STRU p"},
		{parseRetr, "~/Downloads", makeRetrCmd("~/Downloads"), "RETR"},
		{parseRetr, "~/Downloads FAIL?", makeRetrCmd("~/Downloads FAIL?"), "RETR arg with space"},
		{parseStor, "~/Downloads", makeStorCmd("~/Downloads"), "STOR"},
		{parseStor, "~/Downloads FAIL?", makeStorCmd("~/Downloads FAIL?"), "STOR arg with space"},
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
		{parseType, "E D", "TYPE E unrecognized format"},
		{parseType, "L", "TYPE L missing arg"},
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
		{"USER GAVIN\r\n", []FTPCommand{makeUserCmd("GAVIN")}, "USER basic"},
		{"PASS GAVIN\r\n", []FTPCommand{makePassCmd("GAVIN")}, "PASS basic"},
		{"QUIT\r\n", []FTPCommand{makeQuitCmd()}, "QUIT basic"},
		{"PORT 1,1,1,1,000,123\r\n", []FTPCommand{makePortCmd("1.1.1.1", 123)}, "PORT basic"},
		{"TYPE A N\r\n", []FTPCommand{makeTypeCmd('A', 'N', 0)}, "TYPE A N basic"},
		{"TYPE A T\r\n", []FTPCommand{makeTypeCmd('A', 'T', 0)}, "TYPE A T basic"},
		{"TYPE A C\r\n", []FTPCommand{makeTypeCmd('A', 'C', 0)}, "TYPE A C basic"},
		{"TYPE E N\r\n", []FTPCommand{makeTypeCmd('E', 'N', 0)}, "TYPE E N basic"},
		{"TYPE E T\r\n", []FTPCommand{makeTypeCmd('E', 'T', 0)}, "TYPE E T basic"},
		{"TYPE E C\r\n", []FTPCommand{makeTypeCmd('E', 'C', 0)}, "TYPE E C basic"},
		{"TYPE I\r\n", []FTPCommand{makeTypeCmd('I', NONE, 0)}, "TYPE I basic"},
		{"TYPE L 123\r\n", []FTPCommand{makeTypeCmd('L', NONE, 123)}, "TYPE L basic"},
		{"MODE S\r\n", []FTPCommand{makeModeCmd('S')}, "MODE S basic"},
		{"MODE B\r\n", []FTPCommand{makeModeCmd('B')}, "MODE B basic"},
		{"MODE C\r\n", []FTPCommand{makeModeCmd('C')}, "MODE C basic"},
		{"STRU F\r\n", []FTPCommand{makeStruCmd('F')}, "STRU F basic"},
		{"STRU R\r\n", []FTPCommand{makeStruCmd('R')}, "STRU R basic"},
		{"STRU P\r\n", []FTPCommand{makeStruCmd('P')}, "STRU P basic"},
		{"RETR ~/Downloads/cool_file\r\n", []FTPCommand{makeRetrCmd("~/Downloads/cool_file")}, "RETR basic"},
		{"STOR ~/Downloads/cool_file\r\n", []FTPCommand{makeStorCmd("~/Downloads/cool_file")}, "STOR basic"},
		{"UsEr GAVIN\r\n", []FTPCommand{makeUserCmd("GAVIN")}, "Normalize command case"},
		{"NOOP\r\n", []FTPCommand{makeNOOPCmd()}, "NOOP basic"},

		// More complicated strings to parse
		{
			"USER GAVIN\r\nPASS admin\r\n",
			[]FTPCommand{makeUserCmd("GAVIN"), makePassCmd("admin")},
			"complex1",
		},
		{
			"TYPE L 255\r\nSTRU R\r\nSTOR /dev/null\r\nUSER GANDALF\r\n",
			[]FTPCommand{makeTypeCmd('L', NONE, 255), makeStruCmd('R'), makeStorCmd("/dev/null"), makeUserCmd("GANDALF")},
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
		// {"USER GAVIN", "Missing <CRLF>"}, // this fails, do we actually care?
		{"", "Empty input"},
		{"TYPE MODE S\r\n", "Missing arg"},
	}

	for _, tc := range testCases {
		actual, err := Interpret(tc.input)
		if err == nil { // why didnt it fail???
			t.Fatalf("%s case: expected failure, found %v", tc.caseName, actual)
		}
		// log.Fatal(err)
	}
}
