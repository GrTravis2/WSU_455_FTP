package protocolinterpreter

import (
	"slices"
	"testing"
)

func TestParseCommand(t *testing.T) {
	var testCases = []struct {
		parseFunc     func(*FTPCommand, []string) ([]string, error)
		inputWords    []string
		inputCmd      FTPCommand
		expectedWords []string
		expectedCmd   FTPCommand
		caseName      string
	}{
		{parseUser, []string{"Gavin"}, cmdMapping["USER"], []string{}, FTPCommand{USER, UserArgs{"Gavin"}}, "USER"},
		{parseUser, []string{"Gavin", "BANANA"}, cmdMapping["USER"], []string{"BANANA"}, FTPCommand{USER, UserArgs{"Gavin"}}, "USER extra cmd"},
		{parsePass, []string{"secret"}, cmdMapping["PASS"], []string{}, FTPCommand{PASS, PassArgs{"secret"}}, "PASS"},
		{parsePass, []string{"secret", "FUSCHIA"}, cmdMapping["PASS"], []string{"FUSCHIA"}, FTPCommand{PASS, PassArgs{"secret"}}, "PASS extra cmd"},
		{parsePort, []string{"1"}, cmdMapping["PORT"], []string{}, FTPCommand{PORT, PortArgs{1}}, "PORT lower bound"},
		{parsePort, []string{"255"}, cmdMapping["PORT"], []string{}, FTPCommand{PORT, PortArgs{255}}, "PORT upper bound"},
		{parsePort, []string{"255", "SURE"}, cmdMapping["PORT"], []string{"SURE"}, FTPCommand{PORT, PortArgs{255}}, "PORT extra cmd"},
		{parseType, []string{"A", "N"}, cmdMapping["TYPE"], []string{}, FTPCommand{TYPE, TypeArgs{'A', 'N', 0}}, "TYPE A N"},
		{parseType, []string{"A", "T"}, cmdMapping["TYPE"], []string{}, FTPCommand{TYPE, TypeArgs{'A', 'T', 0}}, "TYPE A T"},
		{parseType, []string{"A", "C"}, cmdMapping["TYPE"], []string{}, FTPCommand{TYPE, TypeArgs{'A', 'C', 0}}, "TYPE A C"},
		{parseType, []string{"E", "N"}, cmdMapping["TYPE"], []string{}, FTPCommand{TYPE, TypeArgs{'E', 'N', 0}}, "TYPE E N"},
		{parseType, []string{"E", "T"}, cmdMapping["TYPE"], []string{}, FTPCommand{TYPE, TypeArgs{'E', 'T', 0}}, "TYPE E T"},
		{parseType, []string{"E", "C"}, cmdMapping["TYPE"], []string{}, FTPCommand{TYPE, TypeArgs{'E', 'C', 0}}, "TYPE E C"},
		{parseType, []string{"I", "C"}, cmdMapping["TYPE"], []string{"C"}, FTPCommand{TYPE, TypeArgs{'I', NONE, 0}}, "TYPE I"},
		{parseType, []string{"L", "1"}, cmdMapping["TYPE"], []string{}, FTPCommand{TYPE, TypeArgs{'L', NONE, 1}}, "TYPE L lower bound"},
		{parseType, []string{"L", "255"}, cmdMapping["TYPE"], []string{}, FTPCommand{TYPE, TypeArgs{'L', NONE, 255}}, "TYPE L upper bound"},
		{parseMode, []string{"S"}, cmdMapping["MODE"], []string{}, FTPCommand{MODE, ModeArgs{'S'}}, "MODE S"},
		{parseMode, []string{"B"}, cmdMapping["MODE"], []string{}, FTPCommand{MODE, ModeArgs{'B'}}, "MODE B"},
		{parseMode, []string{"C"}, cmdMapping["MODE"], []string{}, FTPCommand{MODE, ModeArgs{'C'}}, "MODE C"},
		{parseStru, []string{"F"}, cmdMapping["STRU"], []string{}, FTPCommand{STRU, StructureArgs{'F'}}, "STRU F"},
		{parseStru, []string{"R"}, cmdMapping["STRU"], []string{}, FTPCommand{STRU, StructureArgs{'R'}}, "STRU R"},
		{parseStru, []string{"P"}, cmdMapping["STRU"], []string{}, FTPCommand{STRU, StructureArgs{'P'}}, "STRU P"},
		{parseRetr, []string{"~/Downloads"}, cmdMapping["RETR"], []string{}, FTPCommand{RETR, RetrArgs{"~/Downloads"}}, "RETR"},
		{parseRetr, []string{"~/Downloads", "FAIL?"}, cmdMapping["RETR"], []string{"FAIL?"}, FTPCommand{RETR, RetrArgs{"~/Downloads"}}, "RETR extra cmd"},
		{parseStor, []string{"~/Downloads"}, cmdMapping["STOR"], []string{}, FTPCommand{STOR, StorArgs{"~/Downloads"}}, "STOR"},
		{parseRetr, []string{"~/Downloads", "67"}, cmdMapping["RETR"], []string{"67"}, FTPCommand{RETR, RetrArgs{"~/Downloads"}}, "STOR extra cmd"},
	}

	for _, tc := range testCases {
		actual, err := tc.parseFunc(&tc.inputCmd, tc.inputWords)
		if err != nil { // call failed, raise err
			t.Fatal(err)
		}
		if !slices.Equal(actual, tc.expectedWords) { // leftover words wrong
			t.Fatalf("%s case: expected remaining words %v found %v", tc.caseName, tc.expectedWords, actual)
		}
		if tc.inputCmd != tc.expectedCmd { // expected command state wron
			t.Fatalf("%s case: found cmd %#v, expected cmd %#v", tc.caseName, tc.inputCmd, tc.expectedCmd)
		}
	}
}

func TestParseCommandExceptionCases(t *testing.T) {
	var testCases = []struct {
		parseFunc  func(*FTPCommand, []string) ([]string, error)
		inputWords []string
		inputCmd   FTPCommand
		caseName   string
	}{
		{parseUser, []string{}, cmdMapping["USER"], "USER empty"},
		{parsePass, []string{}, cmdMapping["PASS"], "PASS empty"},
		{parsePort, []string{}, cmdMapping["PORT"], "PORT empty"},
		{parsePort, []string{"0"}, cmdMapping["PORT"], "PORT too low"},
		{parsePort, []string{"256"}, cmdMapping["PORT"], "PORT too high"},
		{parsePort, []string{"uint8?"}, cmdMapping["PORT"], "PORT fail uint8 parse"},
		{parseType, []string{}, cmdMapping["TYPE"], "TYPE empty"},
		{parseType, []string{"WALNUT"}, cmdMapping["TYPE"], "TYPE unrecognized type"},
		{parseType, []string{"A"}, cmdMapping["TYPE"], "TYPE A missing arg"},
		{parseType, []string{"A", "Z"}, cmdMapping["TYPE"], "TYPE A unrecognized format"},
		{parseType, []string{"E"}, cmdMapping["TYPE"], "TYPE E missing arg"},
		{parseType, []string{"E", "D"}, cmdMapping["TYPE"], "TYPE E unrecognized format"},
		{parseType, []string{"L"}, cmdMapping["TYPE"], "TYPE L missing arg"},
		{parseType, []string{"L", "0"}, cmdMapping["TYPE"], "TYPE L too low"},
		{parseType, []string{"L", "256"}, cmdMapping["TYPE"], "TYPE L too high"},
		{parseType, []string{"L", "WORD"}, cmdMapping["TYPE"], "TYPE L fail uint8 parse"},
		{parseMode, []string{}, cmdMapping["MODE"], "MODE empty"},
		{parseMode, []string{"R"}, cmdMapping["MODE"], "MODE unrecognized mode"},
		{parseStru, []string{}, cmdMapping["STRU"], "STRU empty"},
		{parseStru, []string{"W"}, cmdMapping["STRU"], "STRU unrecognized mode"},
		{parseRetr, []string{}, cmdMapping["RETR"], "RETR empty"},
		{parseStor, []string{}, cmdMapping["STOR"], "STOR empty"},
	}

	for _, tc := range testCases {
		actual, err := tc.parseFunc(&tc.inputCmd, tc.inputWords)
		if err == nil {
			t.Fatalf("%s case: expected failure, found %#v with leftover words %v", tc.caseName, tc.inputCmd, actual)
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
		{"USER GAVIN \r\n", []FTPCommand{makeUserCmd("GAVIN")}, "USER basic"},
		{"PASS GAVIN \r\n", []FTPCommand{makePassCmd("GAVIN")}, "PASS basic"},
		{"PORT 123 \r\n", []FTPCommand{makePortCmd(123)}, "PORT basic"},
		{"TYPE A N \r\n", []FTPCommand{makeTypeCmd('A', 'N', 0)}, "TYPE A N basic"},
		{"TYPE A T \r\n", []FTPCommand{makeTypeCmd('A', 'T', 0)}, "TYPE A T basic"},
		{"TYPE A C \r\n", []FTPCommand{makeTypeCmd('A', 'C', 0)}, "TYPE A C basic"},
		{"TYPE E N \r\n", []FTPCommand{makeTypeCmd('E', 'N', 0)}, "TYPE E N basic"},
		{"TYPE E T \r\n", []FTPCommand{makeTypeCmd('E', 'T', 0)}, "TYPE E T basic"},
		{"TYPE E C \r\n", []FTPCommand{makeTypeCmd('E', 'C', 0)}, "TYPE E C basic"},
		{"TYPE I \r\n", []FTPCommand{makeTypeCmd('I', NONE, 0)}, "TYPE I basic"},
		{"TYPE L 123 \r\n", []FTPCommand{makeTypeCmd('L', NONE, 123)}, "TYPE L basic"},
		{"MODE S \r\n", []FTPCommand{makeModeCmd('S')}, "MODE S basic"},
		{"MODE B \r\n", []FTPCommand{makeModeCmd('B')}, "MODE B basic"},
		{"MODE C \r\n", []FTPCommand{makeModeCmd('C')}, "MODE C basic"},
		{"STRU F \r\n", []FTPCommand{makeStruCmd('F')}, "STRU F basic"},
		{"STRU R \r\n", []FTPCommand{makeStruCmd('R')}, "STRU R basic"},
		{"STRU P \r\n", []FTPCommand{makeStruCmd('P')}, "STRU P basic"},
		{"RETR ~/Downloads/cool_file \r\n", []FTPCommand{makeRetrCmd("~/Downloads/cool_file")}, "RETR basic"},
		{"STOR ~/Downloads/cool_file \r\n", []FTPCommand{makeStorCmd("~/Downloads/cool_file")}, "STOR basic"},
		{"UsEr GAVIN \r\n", []FTPCommand{makeUserCmd("GAVIN")}, "Normalize command case"},

		// More complicated strings to parse
		{
			"USER GAVIN \r\nPASS admin \r\n",
			[]FTPCommand{makeUserCmd("GAVIN"), makePassCmd("admin")},
			"complex1",
		},
		{
			"TYPE L 255 \r\nSTRU R \r\nSTOR /dev/null \r\nUSER GANDALF\r\n",
			[]FTPCommand{makeTypeCmd('L', NONE, 255), makeStruCmd('R'), makeStorCmd("/dev/null"), makeUserCmd("GANDALF")},
			"complex2",
		},
	}

	for _, tc := range testCases {
		actual, err := Interpret(tc.input)
		if err != nil { // parsing failed!
			t.Fatal(err)
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
		{"USERGAVIN \r\n", "Missing space"},
		// {"USER GAVIN", "Missing <CRLF>"}, // this fails, do we actually care?
		{"", "Empty input"},
		{"TYPE MODE S \r\n", "Missing arg"},
	}

	for _, tc := range testCases {
		actual, err := Interpret(tc.input)
		if err == nil { // why didnt it fail???
			t.Fatalf("%s case: expected failure, found %v", tc.caseName, actual)
		}
		// log.Fatal(err)
	}
}
