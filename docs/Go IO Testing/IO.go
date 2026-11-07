// this is just me going through the io examples and writing them down here to implement later

// Constants

const (
	SeekStart = 0 // seek relative to the origin of the file
	SeekCurrent = 1 // seek relative to the current offset
	SeekEnd = 2 // seek relative to the end
)

// Variables
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

//Functions
// these are some function calls for reading and writing data to and from files, network connections, and other io streams.

// this first function copys from an src to dst until either EOF is reached on src or an error occurs. 
// It returns the number of bytes copied and the first error encountered while copying, if any.
func Copy(dst Writer, src Reader) (written int64, err error)
// A successful Copy returns err == nil, not err == EOF. Because Copy is defined to read from src until EOF, 
// it does not treat an EOF from Read as an error to be reported.


