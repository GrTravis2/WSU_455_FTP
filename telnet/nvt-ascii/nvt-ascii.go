package nvtascii

import (
	"fmt"
	"strings"
)

// NVT ASCII compatible string as outlined in [RFC 854, Telnet Protocol](https://www.rfc-editor.org/info/rfc854/)
type nvtAsciiString []byte

/*
Special NVT ASCII Chars which have special meaning to the NVT per spec
*/

const LF byte = '\x0a' // ascii 10
const CR byte = '\x0d' // ascii 13

// Given a string, validate and turn it into a NVT ASCII Compatible string
func ToNVTAscii(s string) (nvtAsciiString, error) {
	upper := strings.ToUpper(s) // normalize case, do it all in uppercase
	for _, c := range upper {
		if c > 127 {
			fmt_str := "unexpected character '%c', in input '%s'. NVT Ascii is restricted to ascii codes [0-127]"
			return nil, fmt.Errorf(fmt_str, c, s)
		}
	}

	return nvtAsciiString([]byte(upper)), nil
}
