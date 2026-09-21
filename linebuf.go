package linebuf

import (
	"bytes"
	"errors"
	"io"
	"iter"
)

// ErrTokenTooLong is returned when a line exceeds the maximum allowed buffer size.
var ErrTokenTooLong = errors.New("reader: token too long")

type CB func([]byte) error

type Scanner struct {
	StartBufSize int
	MaxBufSize   int
	buf          []byte
	offset       int
	iterErr      error
}

type Option func(*Scanner)

// WithStartBufSize sets the initial buffer size for the scanner.
func WithStartBufSize(size int) Option {
	return func(s *Scanner) {
		s.StartBufSize = size
	}
}

// WithMaxBufSize sets the maximum buffer size for the scanner.
func WithMaxBufSize(size int) Option {
	return func(s *Scanner) {
		s.MaxBufSize = size
	}
}

// Scan reads from the provided reader and invokes the callback for each line.
func Scan(r io.Reader, cb CB, opts ...Option) error {
	s := New(opts...)
	return s.Scan(r, cb)
}

// New creates a new Scanner instance with the provided options.
func New(opts ...Option) *Scanner {
	s := &Scanner{
		StartBufSize: 4096,
		MaxBufSize:   65536,
		offset:       0,
	}
	for _, opt := range opts {
		opt(s)
	}
	s.MaxBufSize = max(s.StartBufSize, s.MaxBufSize)
	s.buf = make([]byte, s.StartBufSize)
	return s
}

var errIterationStopped = errors.New("iteration stopped")

// Iter returns a sequence of lines from the provided reader. Any error encountered during iteration can be retrieved using IterErr.
func (s *Scanner) Iter(r io.Reader) iter.Seq[[]byte] {
	s.iterErr = nil
	return func(yield func([]byte) bool) {
		err := s.Scan(r, func(data []byte) error {
			if !yield(data) {
				return errIterationStopped
			}
			return nil
		})
		if err != nil && err != errIterationStopped { //nolint:errorlint,staticcheck
			s.iterErr = err
		}
	}
}

// IterErr returns the error encountered during iteration, if any. It should be called after Iter to check for errors.
func (s *Scanner) IterErr() error {
	return s.iterErr
}

func (s *Scanner) scanInternal(r io.Reader, cb CB) error {
	nRead, eof, err := s.readInto(r)
	if err != nil {
		return err
	}
	if nRead == 0 && eof {
		if err := s.flushTrailingLine(cb); err != nil {
			return err
		}
		return io.EOF
	}
	n := nRead + s.offset
	if err := s.processChunk(n, cb); err != nil {
		return err
	}

	if eof {
		if err := s.flushTrailingLine(cb); err != nil {
			return err
		}
		return io.EOF
	}

	if s.offset == n {
		if expandErr := s.expand(n); expandErr != nil {
			return expandErr
		}
	}
	return nil
}

func callCB(cb CB, line []byte) error {
	l := len(line)
	if l == 0 {
		return nil
	}
	if line[l-1] == '\r' {
		line = line[:l-1]
	}
	if len(line) == 0 {
		return nil
	}
	return cb(line)
}

// Scan reads from the provided reader and invokes the callback for each line. It returns an error if any occurs during scanning.
func (s *Scanner) Scan(r io.Reader, cb CB) error {
	// reset the offset before starting the scan
	s.offset = 0
	for {
		err := s.scanInternal(r, cb)
		if err != nil && err == io.EOF { //nolint:staticcheck,errorlint
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// flushTrailingLine invokes the callback with any remaining partial line in the buffer.
func (s *Scanner) flushTrailingLine(cb CB) error {
	if s.offset == 0 {
		return nil
	}
	return callCB(cb, s.buf[0:s.offset])
}

// scanNewlines scans the buffer for newline characters and invokes the callback for each complete line. It returns the number of bytes processed and any error encountered.
func (s *Scanner) scanNewlines(n int, cb CB) (int, error) {
	k := 0
	for {
		idx := bytes.IndexByte(s.buf[k:n], '\n')
		if idx < 0 {
			break
		}
		// found newline at k+idx
		if err := callCB(cb, s.buf[k:k+idx]); err != nil {
			return k, err
		}
		k += idx + 1
	}
	return k, nil
}

// compact moves any remaining partial line to the head of the buffer. It updates the offset accordingly.
func (s *Scanner) compact(n, k int) {
	if k < n {
		copy(s.buf[0:], s.buf[k:n])
		s.offset = n - k
	} else {
		s.offset = 0
	}
}

// expand grows the buffer when it is full and contains no newlines. It returns an error if the buffer exceeds the maximum allowed size.
func (s *Scanner) expand(n int) error {
	if n >= s.MaxBufSize {
		return ErrTokenTooLong
	}
	if n == len(s.buf) {
		newSize := len(s.buf) * 2
		newSize = min(newSize, s.MaxBufSize)
		newBuf := make([]byte, newSize)
		copy(newBuf, s.buf)
		s.buf = newBuf
	}
	return nil
}

// readInto performs a single read into the buffer and returns the number of
// bytes read and whether EOF was reached. Non-EOF errors are returned as-is.
func (s *Scanner) readInto(f io.Reader) (int, bool, error) {
	nRead, err := f.Read(s.buf[s.offset:])
	if err == nil {
		return nRead, false, nil
	}
	if err == io.EOF {
		return nRead, true, nil
	}
	return nRead, false, err
}

// processChunk parses all complete lines from the current buffer contents and
// compacts any remaining partial line to the head of the buffer.
func (s *Scanner) processChunk(n int, cb CB) error {
	k, err := s.scanNewlines(n, cb)
	s.compact(n, k)
	return err
}
