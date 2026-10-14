// WSU 455 Semester Project - FTP
// Functions and constants for creating
// NVT ASCII strings.
package nvtascii

import (
	"fmt"
)

// Opaque NVT ASCII compatible string as outlined in [RFC 854, Telnet Protocol](https://www.rfc-editor.org/info/rfc854/)
type nvtAsciiString []byte

/*
Control Chars
NVT ASCII Chars which have special meaning to the NVT per spec
*/

const LF string = "\x0a"       // ascii 10 <LF>
const CR string = "\x0d"       // ascii 13 <CR>
const SP string = "\x20"       // ascii 20 <SP>
const CRLF string = "\x0d\x0a" // <CRLF> == \r\n

// Given a string, validate and turn it into a NVT ASCII Compatible string
func FromString(s string) (nvtAsciiString, error) {
	for _, c := range s {
		if c > 127 { // Assume only ascii codes [0 - 127] valid
			fmt_str := "unexpected character '%c', in input '%s'. NVT Ascii is restricted to ascii codes [0-127]"
			return nil, fmt.Errorf(fmt_str, c, s)
		}
	}
	// normalize string case per FTP spec
	return nvtAsciiString([]byte(s)), nil
}
