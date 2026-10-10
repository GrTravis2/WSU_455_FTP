package nvtascii

import (
	"bytes"
	"log"
	"testing"
)

func TestNVTAsciiRestrictedChars(t *testing.T) {
	var testCases = []struct {
		input    string
		expected nvtAsciiString
		caseName string
	}{
		{"bart", nvtAsciiString("BART"), "normal"},
		{"\x00walter", nvtAsciiString("\x00WALTER"), "lower bound"},
		{"\x7fwalter", nvtAsciiString("\x7fWALTER"), "upper bound"},
		{"BaRt", nvtAsciiString("BART"), "normalize string"},
	}

	for _, tc := range testCases {
		actual, err := ToNVTAscii(tc.input)
		if err != nil {
			log.Fatalf("%v", err)
		}
		if !bytes.Equal([]byte(actual), tc.expected) {
			log.Fatalf(
				"%v case: found '%v', expected '%v'",
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
	}

	for _, tc := range testCases {
		actual, err := ToNVTAscii(tc.input)
		if err == nil { // expected error!!
			log.Fatalf(
				"%v case: expected error on input '%s', found '%v'",
				tc.caseName,
				tc.input,
				actual,
			)
		}
	}
}
