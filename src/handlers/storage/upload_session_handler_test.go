package storage

import (
	"bytes"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

func newTestFileStore(t *testing.T) FileUploadHandler {
	t.Helper()
	return NewFileStore(t.TempDir(), NewMemoryFileStore(), bcrypt.MinCost)
}

func validUpload() FileUpload {
	return FileUpload{
		Filename:         "test.txt",
		ExpiresIn:        time.Hour,
		AllowedDownloads: 5,
	}
}

// --- BeginChunkedUpload ---

func TestBeginChunkedUpload(t *testing.T) {

	t.Run("valid upload creates session", func(t *testing.T) {
		f := newTestFileStore(t)
		id, err := f.BeginChunkedUpload(validUpload(), "alice")
		assert.NoError(t, err)
		assert.NotEqual(t, FileID{}, id)

		_, ok := f.sessionStore.get(id)
		assert.True(t, ok, "session should exist in store")
	})

	t.Run("expiry too short is rejected", func(t *testing.T) {
		f := newTestFileStore(t)
		upload := validUpload()
		upload.ExpiresIn = 30 * time.Second // less than required 1 minute

		_, err := f.BeginChunkedUpload(upload, "alice")
		assert.Error(t, err)
	})

	t.Run("zero downloads is rejected", func(t *testing.T) {
		f := newTestFileStore(t)
		upload := validUpload()
		upload.AllowedDownloads = 0

		_, err := f.BeginChunkedUpload(upload, "alice")
		assert.Error(t, err)
	})

	t.Run("different sessions get different IDs", func(t *testing.T) {
		f := newTestFileStore(t)
		id1, _ := f.BeginChunkedUpload(validUpload(), "alice")
		id2, _ := f.BeginChunkedUpload(validUpload(), "alice")
		assert.NotEqual(t, id1, id2)
	})

	t.Run("temp file is created on disk", func(t *testing.T) {
		f := newTestFileStore(t)
		id, _ := f.BeginChunkedUpload(validUpload(), "alice")

		session, _ := f.sessionStore.get(id)
		_, err := os.Stat(session.TempFile.Name())
		assert.NoError(t, err, "temp file should exist on disk")
	})
}

// --- AppendChunk ---

func TestAppendChunk(t *testing.T) {

	t.Run("chunk is written and byte count returned", func(t *testing.T) {
		f := newTestFileStore(t)
		id, _ := f.BeginChunkedUpload(validUpload(), "alice")

		data := []byte("hello world chunk data")
		n, err := f.AppendChunk(id, "alice", bytes.NewReader(data), int64(len(data)), nil)
		assert.NoError(t, err)
		assert.Equal(t, int64(len(data)), n)
	})

	t.Run("total bytes accumulate across chunks", func(t *testing.T) {
		f := newTestFileStore(t)
		id, _ := f.BeginChunkedUpload(validUpload(), "alice")

		chunk1 := []byte("first chunk")
		chunk2 := []byte("second chunk")
		f.AppendChunk(id, "alice", bytes.NewReader(chunk1), int64(len(chunk1)), nil)
		f.AppendChunk(id, "alice", bytes.NewReader(chunk2), int64(len(chunk2)), nil)

		session, _ := f.sessionStore.get(id)
		assert.Equal(t, int64(len(chunk1)+len(chunk2)), session.TotalBytes)
	})

	t.Run("chunk is capped to limit", func(t *testing.T) {
		f := newTestFileStore(t)
		id, _ := f.BeginChunkedUpload(validUpload(), "alice")

		data := []byte("this is longer than the limit")
		limit := int64(4)
		n, err := f.AppendChunk(id, "alice", bytes.NewReader(data), limit, nil)
		assert.NoError(t, err)
		assert.Equal(t, limit, n)
	})

	t.Run("update callback is called", func(t *testing.T) {
		f := newTestFileStore(t)
		id, _ := f.BeginChunkedUpload(validUpload(), "alice")

		called := 0
		f.AppendChunk(id, "alice", bytes.NewReader([]byte("abc")), 1024, func(n int) { called++ })
		assert.Greater(t, called, 0)
	})

	t.Run("unknown session returns ErrSessionNotFound", func(t *testing.T) {
		f := newTestFileStore(t)
		var unknownID FileID
		_, err := f.AppendChunk(unknownID, "alice", bytes.NewReader(nil), 1024, nil)
		assert.ErrorIs(t, err, ErrSessionNotFound)
	})

	t.Run("wrong username returns ErrSessionNotFound", func(t *testing.T) {
		f := newTestFileStore(t)
		id, _ := f.BeginChunkedUpload(validUpload(), "alice")
		_, err := f.AppendChunk(id, "bob", bytes.NewReader([]byte("data")), 1024, nil)
		assert.ErrorIs(t, err, ErrSessionNotFound)
	})
}

// --- CompleteChunkedUpload ---

func TestCompleteChunkedUpload(t *testing.T) {

	t.Run("returns file ID and removes session", func(t *testing.T) {
		f := newTestFileStore(t)
		id, _ := f.BeginChunkedUpload(validUpload(), "alice")
		f.AppendChunk(id, "alice", bytes.NewReader([]byte("content")), 1024, nil)

		fileId, err := f.CompleteChunkedUpload(id, "alice")
		assert.NoError(t, err)
		assert.NotNil(t, fileId)

		_, ok := f.sessionStore.get(id)
		assert.False(t, ok, "session should be removed after complete")
	})

	t.Run("metadata is stored in file store", func(t *testing.T) {
		f := newTestFileStore(t)
		id, _ := f.BeginChunkedUpload(validUpload(), "alice")
		f.AppendChunk(id, "alice", bytes.NewReader([]byte("content")), 1024, nil)

		fileId, _ := f.CompleteChunkedUpload(id, "alice")
		meta := f.GetFile(*fileId)
		assert.NotNil(t, meta)
		assert.Equal(t, "test.txt", meta.Name)
		assert.Equal(t, int64(7), meta.Size)
	})

	t.Run("unknown session returns ErrSessionNotFound", func(t *testing.T) {
		f := newTestFileStore(t)
		var unknownID FileID
		_, err := f.CompleteChunkedUpload(unknownID, "alice")
		assert.ErrorIs(t, err, ErrSessionNotFound)
	})

	t.Run("wrong username returns ErrSessionNotFound", func(t *testing.T) {
		f := newTestFileStore(t)
		id, _ := f.BeginChunkedUpload(validUpload(), "alice")
		_, err := f.CompleteChunkedUpload(id, "bob")
		assert.ErrorIs(t, err, ErrSessionNotFound)
	})
}

// --- AbortChunkedUpload ---

func TestAbortChunkedUpload(t *testing.T) {

	t.Run("removes session and deletes temp file", func(t *testing.T) {
		f := newTestFileStore(t)
		id, _ := f.BeginChunkedUpload(validUpload(), "alice")

		session, _ := f.sessionStore.get(id)
		tmpPath := session.TempFile.Name()

		err := f.AbortChunkedUpload(id, "alice")
		assert.NoError(t, err)

		_, ok := f.sessionStore.get(id)
		assert.False(t, ok, "session should be removed")

		_, statErr := os.Stat(tmpPath)
		assert.True(t, os.IsNotExist(statErr), "temp file should be deleted")
	})

	t.Run("unknown session returns ErrSessionNotFound", func(t *testing.T) {
		f := newTestFileStore(t)
		var unknownID FileID
		err := f.AbortChunkedUpload(unknownID, "alice")
		assert.ErrorIs(t, err, ErrSessionNotFound)
	})

	t.Run("wrong username leaves session intact", func(t *testing.T) {
		f := newTestFileStore(t)
		id, _ := f.BeginChunkedUpload(validUpload(), "alice")

		err := f.AbortChunkedUpload(id, "bob")
		assert.ErrorIs(t, err, ErrSessionNotFound)

		_, ok := f.sessionStore.get(id)
		assert.True(t, ok, "session should still exist")
	})
}

// --- End-to-end: begin → chunks → complete → ReadFile ---

func TestChunkedUpload_EndToEnd(t *testing.T) {

	data := []byte("the quick brown fox jumps over the lazy dog — chunked upload test data")

	for _, password := range []string{"", "s3cr3t"} {
		for _, store := range getStores(t) {

			t.Run(fmt.Sprintf("password=%q store=%T", password, store), func(t *testing.T) {
				assert := assert.New(t)

				f := NewFileStore(t.TempDir(), store, bcrypt.MinCost)

				upload := FileUpload{
					Filename:         "fox.txt",
					ExpiresIn:        time.Hour,
					AllowedDownloads: 3,
					Password:         password,
				}

				// 1. Begin
				id, err := f.BeginChunkedUpload(upload, "alice")
				assert.NoError(err)

				// 2. Three chunks
				chunkSize := len(data) / 3
				for i := 0; i < len(data); i += chunkSize {
					end := i + chunkSize
					if end > len(data) {
						end = len(data)
					}
					n, err := f.AppendChunk(id, "alice", bytes.NewReader(data[i:end]), int64(len(data)), nil)
					assert.NoError(err)
					assert.Equal(int64(end-i), n)
				}

				// 3. Complete
				fileId, err := f.CompleteChunkedUpload(id, "alice")
				assert.NoError(err)
				assert.NotNil(fileId)

				// 4. Verify metadata
				meta := f.GetFile(*fileId)
				assert.NotNil(meta)
				assert.Equal("fox.txt", meta.Name)
				assert.Equal(int64(len(data)), meta.Size)
				assert.False(meta.IsExpired())

				// 5. Verify content round-trips correctly
				var buf bytes.Buffer
				err = f.ReadFile(&buf, *fileId, password)
				assert.NoError(err)
				assert.Equal(data, buf.Bytes())
			})
		}
	}
}

// --- Session expiry sweep ---

func TestUploadSessionStore_ClearExpired(t *testing.T) {

	t.Run("expired sessions are swept and temp files deleted", func(t *testing.T) {
		f := newTestFileStore(t)

		// Begin a session, then manually back-date its LastActivity.
		id, _ := f.BeginChunkedUpload(validUpload(), "alice")

		session, _ := f.sessionStore.get(id)
		tmpPath := session.TempFile.Name()
		session.LastActivity = time.Now().Add(-2 * time.Hour) // far in the past

		f.sessionStore.timeout = time.Minute
		f.sessionStore.ClearExpired()

		_, ok := f.sessionStore.get(id)
		assert.False(t, ok, "expired session should be removed")

		_, statErr := os.Stat(tmpPath)
		assert.True(t, os.IsNotExist(statErr), "temp file should be deleted")
	})

	t.Run("active sessions are not swept", func(t *testing.T) {
		f := newTestFileStore(t)
		id, _ := f.BeginChunkedUpload(validUpload(), "alice")

		f.sessionStore.timeout = time.Minute
		f.sessionStore.ClearExpired()

		_, ok := f.sessionStore.get(id)
		assert.True(t, ok, "active session should not be swept")
	})
}
