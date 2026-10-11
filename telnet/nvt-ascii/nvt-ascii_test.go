package nvtascii

import (
	"bytes"
	"testing"
)

func TestNVTAsciiRestrictedChars(t *testing.T) {
	var testCases = []struct {
		input    string
		expected nvtAsciiString
		caseName string
	}{
		{"bart", nvtAsciiString("BART"), "basic"},
		{"USER bart" + SP + CRLF, nvtAsciiString("USER BART \r\n"), "user FTP cmd example"},
		{"STOU" + SP + CRLF, nvtAsciiString("STOU \r\n"), "store unique FTP cmd example"},
		{"\x00walter", nvtAsciiString("\x00WALTER"), "lower bound"},
		{"\x7fwalter", nvtAsciiString("\x7fWALTER"), "upper bound"},
		{"BaRt", nvtAsciiString("BART"), "normalize string"},
	}

	for _, tc := range testCases {
		actual, err := FromString(tc.input)
		if err != nil {
			t.Fatalf("%v", err)
		}
		if !bytes.Equal([]byte(actual), tc.expected) {
			t.Fatalf(
				"%v case: found '%s', expected '%s'",
				tc.caseName,
				actual,
				tc.expected,
			)
		}
	}
}

func TestNVTAsciiExceptionCases(t *testing.T) {
	var testCases = []struct {
		input    string
		caseName string
	}{
		{"😳", "illegal char"},
		{"\xff", "underflow"},
		{"\x80", "upper bound"},
		{"\u017f", "weird upper conv to 'S' should fail"},
	}

	for _, tc := range testCases {
		actual, err := FromString(tc.input)
		if err == nil { // expected error!!
			t.Fatalf(
				"%v case: expected error on input '%s', found '%s'",
				tc.caseName,
				tc.input,
				actual,
			)
		}
	}
}
