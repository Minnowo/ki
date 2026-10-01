package storage

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type closeTracker struct {
	*strings.Reader
	Closed bool
}

func (c *closeTracker) Close() error {
	c.Closed = true
	return nil
}

func (c *closeTracker) SeekTo(offset int64) error {
	_, err := c.Seek(offset, io.SeekStart)
	return err
}

func TestDownloadSessionClose(t *testing.T) {

	stream := &closeTracker{Reader: strings.NewReader("data")}
	session := &DownloadSession{Stream: stream}

	assert.Nil(t, session.Close(), "close should not error")
	assert.True(t, stream.Closed, "close should close the underlying stream")
}

func TestDownloadSessionStoreRemoveExpired(t *testing.T) {

	store := newDownloadSessionStore(time.Minute)

	var expiredID, activeID SessionToken
	expiredID.New()
	activeID.New()

	expiredStream := &closeTracker{Reader: strings.NewReader("data")}
	activeStream := &closeTracker{Reader: strings.NewReader("data")}

	expired := &DownloadSession{Stream: expiredStream}
	expired.LastActivity.Set(time.Now().Add(-time.Hour))

	active := &DownloadSession{Stream: activeStream}
	active.LastActivity.Touch()

	store.add(expiredID, expired)
	store.add(activeID, active)

	store.removeExpired()

	_, ok := store.get(expiredID)
	assert.False(t, ok, "expired session should be removed")
	assert.True(t, expiredStream.Closed, "expired session stream should be closed")

	_, ok = store.get(activeID)
	assert.True(t, ok, "active session should remain")
	assert.False(t, activeStream.Closed, "active session stream should stay open")
}

func TestDownloadSessionWriteToNExact(t *testing.T) {

	stream := &closeTracker{Reader: strings.NewReader("abcdefgh")}

	// buffer size divides the data evenly, so pending is empty exactly when size runs out
	session := &DownloadSession{Stream: stream, TotalBytes: 8, buf: make([]byte, 4)}

	var out strings.Builder

	n, err := session.WriteToN(&out, 4)
	assert.Nil(t, err, "writing part of the stream should not error")
	assert.Equal(t, int64(4), n)

	n, err = session.WriteToN(&out, 4)
	assert.Nil(t, err, "writing exactly the rest of the stream should not error")
	assert.Equal(t, int64(4), n)

	assert.Equal(t, "abcdefgh", out.String())
	assert.Equal(t, int64(8), session.BytesWritten)

	_, err = session.WriteToN(&out, 1)
	assert.ErrorIs(t, err, io.EOF, "reading past the end should return EOF")
}
