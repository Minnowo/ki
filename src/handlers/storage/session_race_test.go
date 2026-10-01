package storage

import (
	"bytes"
	"io"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// These tests run the session sweepers concurrently with in-flight requests.
// Run them with the race detector to see the unsynchronized access to the sessions:
//
//	go test -race -run Race ./src/handlers/storage/

// slowReader returns size bytes per Read, sleeping delay before each, for count reads.
type slowReader struct {
	count int
	size  int
	delay time.Duration
}

func (r *slowReader) Read(p []byte) (int, error) {

	if r.count <= 0 {
		return 0, io.EOF
	}

	time.Sleep(r.delay)
	r.count--

	n := min(len(p), r.size)
	copy(p, bytes.Repeat([]byte{'a'}, n))

	return n, nil
}

// slowStream is a slowReader with a Close, which fails reads after being closed.
type slowStream struct {
	slowReader
	closed atomic.Bool
}

func (s *slowStream) Read(p []byte) (int, error) {

	if s.closed.Load() {
		return 0, os.ErrClosed
	}

	return s.slowReader.Read(p)
}

func (s *slowStream) Close() error {
	s.closed.Store(true)
	return nil
}

// sweepUntil calls sweep in a loop until done is closed.
func sweepUntil(done <-chan struct{}, sweep func()) <-chan struct{} {

	stopped := make(chan struct{})

	go func() {
		defer close(stopped)

		for {
			select {
			case <-done:
				return
			default:
				sweep()
				time.Sleep(5 * time.Millisecond)
			}
		}
	}()

	return stopped
}

// A single chunk which takes longer than the session timeout, but is still actively receiving data,
// must not be swept out from under the upload.
func TestRace_UploadSessionSweptDuringChunk(t *testing.T) {

	f := newTestFileStore(t)
	f.uploadSessionStore.timeout = 200 * time.Millisecond

	id, err := f.CreateUploadSession(validUpload(), "alice")
	assert.NoError(t, err)

	// 20 reads * 20ms = 400ms total, well past the timeout, but never idle for longer than 20ms.
	body := &slowReader{count: 20, size: 1024, delay: 20 * time.Millisecond}

	done := make(chan struct{})
	stopped := sweepUntil(done, f.uploadSessionStore.ClearExpired)

	n, err := f.UploadSessionData(id, "alice", body, 1<<20, nil)

	close(done)
	<-stopped

	assert.NoError(t, err, "active upload should not be swept mid-chunk")
	assert.Equal(t, int64(20*1024), n)

	_, ok := f.uploadSessionStore.get(id)
	assert.True(t, ok, "active upload session should still exist")
}

// A download chunk being served while the sweeper runs.
// The session stays active the whole time, so it should never be swept,
// and the sweeper must not touch it while the request holds the session lock.
func TestRace_DownloadSessionSweptDuringChunk(t *testing.T) {

	f := newTestFileStore(t)
	f.downloadSessionStore.timeout = 200 * time.Millisecond

	const reads = 20
	const size = 1024

	stream := &slowStream{slowReader: slowReader{count: reads, size: size, delay: 20 * time.Millisecond}}

	var sessionID SessionToken
	sessionID.New()

	fileID := FileID{1}

	session := &DownloadSession{
		Stream:     stream,
		TotalBytes: reads * size,
		FileID:     fileID,
		buf:        make([]byte, size),
	}
	session.LastActivity.Touch()

	f.downloadSessionStore.add(sessionID, session)

	done := make(chan struct{})
	stopped := sweepUntil(done, f.downloadSessionStore.removeExpired)

	var out bytes.Buffer

	err := f.WithDownloadSession(sessionID, fileID, func(session *DownloadSession) error {
		_, err := session.WriteToN(&out, reads*size)
		return err
	})

	close(done)
	<-stopped

	assert.NoError(t, err, "active download should not be swept mid-chunk")
	assert.Equal(t, reads*size, out.Len())
}

// A request which got a session from the store just before the sweeper closed it
// must see that the session is gone once it gets the lock.
func TestRace_ClosedSessionIsNotUsed(t *testing.T) {

	t.Run("download", func(t *testing.T) {
		f := newTestFileStore(t)

		stream := &slowStream{slowReader: slowReader{count: 1, size: 1024}}

		var sessionID SessionToken
		sessionID.New()

		fileID := FileID{1}

		session := &DownloadSession{Stream: stream, TotalBytes: 1024, FileID: fileID, buf: make([]byte, 1024)}
		session.LastActivity.Touch()

		// closed, but still in the store, as if the sweeper closed it after the request's get()
		f.downloadSessionStore.add(sessionID, session)
		session.Close()

		called := false
		err := f.WithDownloadSession(sessionID, fileID, func(*DownloadSession) error {
			called = true
			return nil
		})

		assert.ErrorIs(t, err, ErrSessionNotFound)
		assert.False(t, called, "callback should not run on a closed session")
		assert.ErrorIs(t, f.AbortDownloadSession(sessionID, fileID), ErrSessionNotFound)
	})

	t.Run("upload", func(t *testing.T) {
		f := newTestFileStore(t)

		id, err := f.CreateUploadSession(validUpload(), "alice")
		assert.NoError(t, err)

		// cleaned up, but still in the store, as if the sweeper cleaned it after the request's get()
		session, _ := f.uploadSessionStore.get(id)
		session.Cleanup()

		_, err = f.UploadSessionData(id, "alice", bytes.NewReader([]byte("data")), 1024, nil)
		assert.ErrorIs(t, err, ErrSessionNotFound)

		_, err = f.CompleteUploadSession(id, "alice")
		assert.ErrorIs(t, err, ErrSessionNotFound)

		assert.ErrorIs(t, f.AbortUploadSession(id, "alice"), ErrSessionNotFound)
	})
}
