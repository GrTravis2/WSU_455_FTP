// WSU 455 Semester Project - FTP
// FTP Server Tests
package server

import (
	"testing"
	nvtascii "wsu455ftp/telnet/nvt-ascii"
)

func TestFTPReplyToString(t *testing.T) {
	var testCases = []struct {
		input    FTPReply
		expected string
		caseName string
	}{
		{FTPReply{[3]byte{2, 2, 0}, "Service ready"}, "220 Service ready\r\n", "220 greeting"},
		{FTPReply{[3]byte{2, 0, 0}, "Command okay"}, "200 Command okay\r\n", "200 command success"},
		{FTPReply{[3]byte{2, 2, 1}, "Service closing control connection"}, "221 Service closing control connection\r\n", "221 connection closing"},
		{FTPReply{[3]byte{5, 0, 0}, "Syntax error, command unrecognized"}, "500 Syntax error, command unrecognized\r\n", "500 error"},
	}

	for _, tc := range testCases {
		actual := tc.input.ToString()
		if actual != tc.expected {
			t.Fatalf("%s case: found %q, expected %q", tc.caseName, actual, tc.expected)
		}
		b, err := nvtascii.FromString(actual)
		if err != nil {
			t.Fatalf("%s case: %v", tc.caseName, err.Error())
		}
		if string(b) != tc.expected {
			t.Fatalf("%s case: found NVT ASCII %q, expected %q", tc.caseName, b, tc.expected)
		}
	}
}
