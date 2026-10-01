package storage

import (
	"errors"
	"io"
	"sync"
)

var ErrInvalidSeek = errors.New("invalid seek start position")

// FileStream is a decrypted file which can be moved to any position.
type FileStream interface {
	io.ReadCloser
	SeekTo(offset int64) error
}

// DownloadSession holds an open decryption reader for a chunked download.
//
// The session counts as one download, so it must not deliver the file more than once.
// Chunks are read in order, and only the last chunk can be read again, so a client can retry a chunk which
// failed to arrive. See StartChunk.
type DownloadSession struct {
	mu sync.Mutex

	// Open decryption reader; stays alive between chunk requests.
	Stream FileStream

	// Total file size, reported to the client so it knows when all chunks are received.
	TotalBytes int64

	// Bytes delivered to the client so far. This is the position of the next byte WriteToN will write.
	BytesWritten int64

	// chunkStart is where the last chunk started. A new chunk cannot start before this.
	chunkStart int64

	// seekFailed is set if Stream failed to seek, so its position no longer matches BytesWritten.
	seekFailed bool

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

// StartChunk moves the session to start, where the next chunk begins.
//
// Moving forward starts a new chunk. Moving back is a retry, and is only allowed as far as the start of the last chunk,
// otherwise a client could read the whole file again from one session.
// Returns ErrInvalidSeek if start is before the last chunk or past the end of the file.
func (session *DownloadSession) StartChunk(start int64) error {

	if start < session.chunkStart || start > session.TotalBytes {
		return ErrInvalidSeek
	}

	isRetry := start < session.BytesWritten

	if start != session.BytesWritten || session.seekFailed {

		if err := session.Stream.SeekTo(start); err != nil {
			session.seekFailed = true
			return err
		}

		session.seekFailed = false
		session.BytesWritten = start
		session.pending = nil
	}

	// a retry keeps the last chunk's start, so it can be retried again
	if !isRetry {
		session.chunkStart = start
	}

	return nil
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
