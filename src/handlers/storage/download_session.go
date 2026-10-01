package storage

import (
	"errors"
	"io"
	"sync"
)

var ErrInvalidSeek = errors.New("invalid seek start position")

// DownloadSession holds an open decryption reader for a chunked download.
// The AES-CTR stream state is maintained naturally as the reader is consumed
// sequentially across multiple chunk requests.
type DownloadSession struct {
	mu sync.Mutex

	// Open decryption reader; stays alive between chunk requests.
	Stream io.ReadCloser

	// Total file size, reported to the client so it knows when all chunks are received.
	TotalBytes int64

	// Bytes delivered to the client so far.
	BytesWritten int64

	FileID FileID
	File   KiMetadata

	// LastActivity can be read without holding mu.
	LastActivity activityClock

	// closed is set once Stream is closed. A request which got this session from the store
	// before it was closed must check this after taking mu.
	closed bool

	// buf is a fixed size buffer containing the last read data from the stream.
	buf []byte

	// pending, if len() > 0, is a slice of buf containing the next data to read.
	// When pending is nil or has len() == 0, buf should be updated with new data and pending should be updated.
	pending []byte
}

// Close closes the stream. The caller must hold mu.
func (session *DownloadSession) Close() error {

	if session.closed {
		return nil
	}

	session.closed = true

	return session.Stream.Close()
}

// SeekUntil reads the session download until the given position.
// The position must be ahead or equal to the current position, otherwise ErrInvalidSeek is returned.
func (session *DownloadSession) SeekUntil(start int64) (int64, error) {

	if start < 0 || start < session.BytesWritten {
		return 0, ErrInvalidSeek
	}

	if session.BytesWritten == start {
		return 0, nil
	}

	return session.WriteToN(io.Discard, start-session.BytesWritten)
}

// WriteToN writes size bytes into the given Writer, stopping when there is an error or it has written size bytes.
// Returns 0 <= n <= size and any error encountered.
func (session *DownloadSession) WriteToN(w io.Writer, size int64) (int64, error) {

	amnt := int64(0)
	for {
		// checked before reading, so writing exactly the remaining bytes doesn't read past them
		if size <= 0 {
			return amnt, nil
		}

		if pLen := len(session.pending); pLen > 0 {

			end := int(min(size, int64(pLen)))
			m, err := w.Write(session.pending[0:end])

			amnt += int64(m)
			size -= int64(m)

			session.BytesWritten += int64(m)
			session.LastActivity.Touch()
			session.pending = session.pending[m:]

			if err != nil {
				return amnt, err
			}

			continue
		}

		m, err := session.Stream.Read(session.buf)
		session.pending = session.buf[0:m]

		if m > 0 {
			continue
		}

		if err == nil {
			err = io.EOF
		}

		return amnt, err
	}
}
