// this is just me going through the io examples and writing them down here to implement later

////
//// Constants
////

const (
	SeekStart = 0 // seek relative to the origin of the file
	SeekCurrent = 1 // seek relative to the current offset
	SeekEnd = 2 // seek relative to the end
)

////
//// Variables
////

// there are several error types here
var EOF = errors.New("EOF") // EOF is returned by read when there is no more input available.
// this error coveres when someone is reading or writing operations to a closed pipe
var ErrClosedPipe = errors.New("IO: read/write on closed pipe")
// more understanding on pipe errors 
// https://stackoverflow.com/questions/77963594/io-read-write-on-closed-pipe-when-copying-large-data


// this is returned by some clients of a reader when many calls to read have failed to return any data or errors, this can be a sign of a broken reader
// implementation.
var ErrNoProgress = errors.New("Multiple read calls return no data or error") 

// the short buffer error means that a read required a longer buffer then what was provided to the reader, 
// this is a common error when reading from a reader that has a fixed size buffer, such as a file or network connection.
var ErrShortBuffer = errors.New("Short buffer")

// this error occurs when the end of file is reached in the middle of reading a fixed size block or data structure, 
// such as a struct or array. This can happen when reading from a file or network connection that is truncated or corrupted.
var ErrUnexpectedEOF = errors.New("Unexpected End of File(EOF) occured in the middle of reading a fixed size block or data structure")

////
//// Functions
////

// these are some function calls for reading and writing data to and from files, network connections, and other io streams.

/// this first function copys from an src to dst until either EOF is reached on src or an error occurs. 
// It returns the number of bytes copied and the first error encountered while copying, if any.
func Copy(dst Writer, src Reader) (written int64, err error)
// A successful Copy returns err == nil, not err == EOF. Because Copy is defined to read from src until EOF, 
// it does not treat an EOF from Read as an error to be reported.

// CopyBuffer is identical to copy except that it stages through the provided buffer (if one is required) rather than allocating a temporary one.
// if buffer is nil, one is allocated; otherwise, if it has zero length, CopyBuffer panics.
func CopyBuffer(dst Writer, src Reader, buf []byte) (written int64, err error)
// if either src implements WriteerTo or dist implements ReaderForm, buffer will not be used to perform the copy.

/// CopyN copies n bytes (or until an error) from src to dst. It returns the number of bytes copied and the earliers error encountered while copying
// on return, written == n if and only if err == nil.
func CopyN(dst Writer, src Reader, n int64) (written int64, err error)
// if dst implements ReaderForm, the copy is implemented using it.
///
/// Example of copy
///

// package main

// import (
// 	"io"
// 	"log"
// 	"os"
// 	"strings"
// )

// func main()	{
// 	r := strings.NewReader("some io.Reader stream to be read")
// 	if _, err := io.Copy(os.Stdout, r); err != nil {
// 		log.Fatal(err)
// 	}
// }
/// Output Result: some

/// func Pipe
// Pipe creates a synchronous in-memory pipe. It can be used to connect code expecting an |io.Reader| with code expecting an |io.Writer|.
// Reads and writes on the pipe are matched one to one except when multiple Reads are needed to consume a single Write. Each Write
// to the PipeWriter blocks until it has satisfied one or more Reads from the PipeReader that fully consume the written data. 
// the Data is copied directly from the Write to the corresponding Read(or Reads); there is no internal buffering.

func Pipe() (*PipeReader, *PipeWriter)

// it is safe to call Read and Write in parallel with each other or with Close. Parallel calls to Read and parallel calls to Write are also safe:
// the individual calls will be gated sequentially.

/// Example of pipe
// package main

// import (
// 	"fmt"
// 	"io"
// 	"log"
// 	"os"
// )
// func main(){
// 	r, w := io.Pipe()

// 	go func() {
// 		fmt.Fprint(w, "some io.Reader stream to be read\n")
// 		w.Close()
// 	}()

// 	if _, err := io.Copy(os.Stdout, r); err != nil {
// 		log.Fatal(err)
// 	}

// }

// Output Result: some io.Reader stream to be read.

// func ReadAtLeast
// ReadAtLeast reads from r into buf until it has read at least min bytes. It returns the number of bytes copied and an error in fewer bytes were read
// The error is EOF only if no bytes were read.

func ReadAtLeast(r Reader, buf []byte, min int) (n int, err error)

// if an EOF happens after reading fewer than min bytes, ReadAtLeast returns ErrUnexpectedEOF. If min is greater than the length of buf, ReadAtLeast returns ErrShortBuffer.
// on return, n >= min if and only if err == nil. If r returns an error having read at least min bytes, the error is dropped.


/// func ReadFull
// ReadFull reads exactly len(buf) bytes from r into buf.
func ReadFull(r Reader, buf []byte) (n int, err error)
// it returns the number of bytes copied and an error if fewer bytes were read.
// the error is EOF only if no bytes were read. If an EOF happens after reading fewer than len(buf) bytes, ReadFull returns ErrUnexpectedEOF.
// On return, n == len(buf) if and only if err == nil. If r returns an error having read at least len(buf) bytes, the error is dropped.

/// WriteString
// WriteString writes the contents of the string s to w, which accepts a slice of bytes. if w implements StringWriter, StringWriter.WriteString
// is invoked directly. Otherwise, Writer.Write is called exactly once.
Func WriteString(w Writer, s string) (n int, err error)

////
//// Types
////

/// Type ByteReader
// ByteReader is the interface that wraps the ReadByte method.
type ByteReader interface {
	ReadByte() (byte, error)
}
// Readbyte reads and returns the next byte from the input or any error encountered. if ReadByte returns an error, no input byte was consumed,
// and the returned byte valuye is undefined.

/// Type ByteScanner
// ByteScanner is the interface that adds the UnreadByte method to the basic ReadByte method.
type ByteScanner interface {
	ByteReader
	UnreadByte() error
}
// UnreadByte causes the next call to ReadByte to return the last byte read. If the last operation was not a successful call to ReadByte,
// UnreadByte may return an error, unread the least byte read(or the byte prior to the last unread byte), or (in implementations that support the Seeker Interface)
// seek to one byte before the current offset.

/// Type ByteWriter
// ByteWriter is the interface that wraps the WriteByte method.
type ByteWriter interface {
	WriteByte(c byte) error
}

/// Type Closer
// Closer is the interface that wraps the basic Close method
type Closer interface {
	Close() error
}
// The behavior of Close after the first call is undefined. Specific implementations may document their own behavior.

/// Type LimitedReader
// LimitedReader reads from R but limits the amount of data returned to just N bytes. Each call to Read updates N to reflect the new amount remaining.
// Read returns EOF when N <= 0 or when the underlying R returns EOF.
type LimitedReader struct {
	R Reader // underlying reader
	N int64 // max bytes remaining
}

