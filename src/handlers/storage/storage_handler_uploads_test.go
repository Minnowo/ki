package storage

import (
	"bytes"
	"fmt"
	"ki/src/config"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

func init() {
	config.InitLogging()
}

func newTestFileStore(t *testing.T) StorageHandler {
	t.Helper()
	return NewStorageHandler(t.TempDir(), NewMemoryFileStore(), bcrypt.MinCost)
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
		id, err := f.CreateUploadSession(validUpload(), "alice")
		assert.NoError(t, err)
		assert.NotEqual(t, FileID{}, id)

		_, ok := f.uploadSessionStore.get(id)
		assert.True(t, ok, "session should exist in store")
	})

	t.Run("expiry too short is rejected", func(t *testing.T) {
		f := newTestFileStore(t)
		upload := validUpload()
		upload.ExpiresIn = 30 * time.Second // less than required 1 minute

		_, err := f.CreateUploadSession(upload, "alice")
		assert.Error(t, err)
	})

	t.Run("zero downloads is rejected", func(t *testing.T) {
		f := newTestFileStore(t)
		upload := validUpload()
		upload.AllowedDownloads = 0

		_, err := f.CreateUploadSession(upload, "alice")
		assert.Error(t, err)
	})

	t.Run("different sessions get different IDs", func(t *testing.T) {
		f := newTestFileStore(t)
		id1, _ := f.CreateUploadSession(validUpload(), "alice")
		id2, _ := f.CreateUploadSession(validUpload(), "alice")
		assert.NotEqual(t, id1, id2)
	})

	t.Run("temp file is created on disk", func(t *testing.T) {
		f := newTestFileStore(t)
		id, _ := f.CreateUploadSession(validUpload(), "alice")

		session, _ := f.uploadSessionStore.get(id)
		_, err := os.Stat(session.TempFile.Name())
		assert.NoError(t, err, "temp file should exist on disk")
	})
}

// --- AppendChunk ---

func TestAppendChunk(t *testing.T) {

	t.Run("chunk is written and byte count returned", func(t *testing.T) {
		f := newTestFileStore(t)
		id, _ := f.CreateUploadSession(validUpload(), "alice")

		data := []byte("hello world chunk data")
		n, err := f.UploadSessionData(id, "alice", bytes.NewReader(data), int64(len(data)), nil)
		assert.NoError(t, err)
		assert.Equal(t, int64(len(data)), n)
	})

	t.Run("total bytes accumulate across chunks", func(t *testing.T) {
		f := newTestFileStore(t)
		id, _ := f.CreateUploadSession(validUpload(), "alice")

		chunk1 := []byte("first chunk")
		chunk2 := []byte("second chunk")
		f.UploadSessionData(id, "alice", bytes.NewReader(chunk1), int64(len(chunk1)), nil)
		f.UploadSessionData(id, "alice", bytes.NewReader(chunk2), int64(len(chunk2)), nil)

		session, _ := f.uploadSessionStore.get(id)
		assert.Equal(t, int64(len(chunk1)+len(chunk2)), session.TotalBytes)
	})

	t.Run("chunk is capped to limit", func(t *testing.T) {
		f := newTestFileStore(t)
		id, _ := f.CreateUploadSession(validUpload(), "alice")

		data := []byte("this is longer than the limit")
		limit := int64(4)
		n, err := f.UploadSessionData(id, "alice", bytes.NewReader(data), limit, nil)
		assert.NoError(t, err)
		assert.Equal(t, limit, n)
	})

	t.Run("update callback is called", func(t *testing.T) {
		f := newTestFileStore(t)
		id, _ := f.CreateUploadSession(validUpload(), "alice")

		called := 0
		f.UploadSessionData(id, "alice", bytes.NewReader([]byte("abc")), 1024, func(n int) { called++ })
		assert.Greater(t, called, 0)
	})

	t.Run("unknown session returns ErrSessionNotFound", func(t *testing.T) {
		f := newTestFileStore(t)
		var unknownID SessionToken
		_, err := f.UploadSessionData(unknownID, "alice", bytes.NewReader(nil), 1024, nil)
		assert.ErrorIs(t, err, ErrSessionNotFound)
	})

	t.Run("wrong username returns ErrSessionNotFound", func(t *testing.T) {
		f := newTestFileStore(t)
		id, _ := f.CreateUploadSession(validUpload(), "alice")
		_, err := f.UploadSessionData(id, "bob", bytes.NewReader([]byte("data")), 1024, nil)
		assert.ErrorIs(t, err, ErrSessionNotFound)
	})
}

// --- CompleteChunkedUpload ---

func TestCompleteChunkedUpload(t *testing.T) {

	t.Run("returns file ID and removes session", func(t *testing.T) {
		f := newTestFileStore(t)
		id, _ := f.CreateUploadSession(validUpload(), "alice")
		f.UploadSessionData(id, "alice", bytes.NewReader([]byte("content")), 1024, nil)

		fileId, err := f.CompleteUploadSession(id, "alice")
		assert.NoError(t, err)
		assert.NotNil(t, fileId)

		_, ok := f.uploadSessionStore.get(id)
		assert.False(t, ok, "session should be removed after complete")
	})

	t.Run("metadata is stored in file store", func(t *testing.T) {
		f := newTestFileStore(t)
		id, _ := f.CreateUploadSession(validUpload(), "alice")
		f.UploadSessionData(id, "alice", bytes.NewReader([]byte("content")), 1024, nil)

		fileId, _ := f.CompleteUploadSession(id, "alice")
		meta := f.FileMetadata(*fileId)
		assert.NotNil(t, meta)
		assert.Equal(t, "test.txt", meta.Name)
		assert.Equal(t, int64(7), meta.Size)
	})

	t.Run("unknown session returns ErrSessionNotFound", func(t *testing.T) {
		f := newTestFileStore(t)
		var unknownID SessionToken
		_, err := f.CompleteUploadSession(unknownID, "alice")
		assert.ErrorIs(t, err, ErrSessionNotFound)
	})

	t.Run("wrong username returns ErrSessionNotFound", func(t *testing.T) {
		f := newTestFileStore(t)
		id, _ := f.CreateUploadSession(validUpload(), "alice")
		_, err := f.CompleteUploadSession(id, "bob")
		assert.ErrorIs(t, err, ErrSessionNotFound)
	})
}

// --- AbortChunkedUpload ---

func TestAbortChunkedUpload(t *testing.T) {

	t.Run("removes session and deletes temp file", func(t *testing.T) {
		f := newTestFileStore(t)
		id, _ := f.CreateUploadSession(validUpload(), "alice")

		session, _ := f.uploadSessionStore.get(id)
		tmpPath := session.TempFile.Name()

		err := f.AbortUploadSession(id, "alice")
		assert.NoError(t, err)

		_, ok := f.uploadSessionStore.get(id)
		assert.False(t, ok, "session should be removed")

		_, statErr := os.Stat(tmpPath)
		assert.True(t, os.IsNotExist(statErr), "temp file should be deleted")
	})

	t.Run("unknown session returns ErrSessionNotFound", func(t *testing.T) {
		f := newTestFileStore(t)
		var unknownID SessionToken
		err := f.AbortUploadSession(unknownID, "alice")
		assert.ErrorIs(t, err, ErrSessionNotFound)
	})

	t.Run("wrong username leaves session intact", func(t *testing.T) {
		f := newTestFileStore(t)
		id, _ := f.CreateUploadSession(validUpload(), "alice")

		err := f.AbortUploadSession(id, "bob")
		assert.ErrorIs(t, err, ErrSessionNotFound)

		_, ok := f.uploadSessionStore.get(id)
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

				f := NewStorageHandler(t.TempDir(), store, bcrypt.MinCost)

				upload := FileUpload{
					Filename:         "fox.txt",
					ExpiresIn:        time.Hour,
					AllowedDownloads: 3,
					Password:         password,
				}

				// 1. Begin
				id, err := f.CreateUploadSession(upload, "alice")
				assert.NoError(err)

				// 2. Three chunks
				chunkSize := len(data) / 3
				for i := 0; i < len(data); i += chunkSize {
					end := min(i+chunkSize, len(data))
					n, err := f.UploadSessionData(id, "alice", bytes.NewReader(data[i:end]), int64(len(data)), nil)
					assert.NoError(err)
					assert.Equal(int64(end-i), n)
				}

				// 3. Complete
				fileId, err := f.CompleteUploadSession(id, "alice")
				assert.NoError(err)
				assert.NotNil(fileId)

				// 4. Verify metadata
				meta := f.FileMetadata(*fileId)
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
		id, _ := f.CreateUploadSession(validUpload(), "alice")

		session, _ := f.uploadSessionStore.get(id)
		tmpPath := session.TempFile.Name()
		session.LastActivity = time.Now().Add(-2 * time.Hour) // far in the past

		f.uploadSessionStore.timeout = time.Minute
		f.uploadSessionStore.ClearExpired()

		_, ok := f.uploadSessionStore.get(id)
		assert.False(t, ok, "expired session should be removed")

		_, statErr := os.Stat(tmpPath)
		assert.True(t, os.IsNotExist(statErr), "temp file should be deleted")
	})

	t.Run("active sessions are not swept", func(t *testing.T) {
		f := newTestFileStore(t)
		id, _ := f.CreateUploadSession(validUpload(), "alice")

		f.uploadSessionStore.timeout = time.Minute
		f.uploadSessionStore.ClearExpired()

		_, ok := f.uploadSessionStore.get(id)
		assert.True(t, ok, "active session should not be swept")
	})
}

func TestFullUpload(t *testing.T) {

	for _, store := range getStores(t) {

		for _, useMemoOnly := range []bool{false, true} {

			t.Run("test read & write", func(t *testing.T) {

				assert := assert.New(t)
				tempDir := t.TempDir()

				filemap := NewStorageHandler(tempDir, store, bcrypt.MinCost)

				data := []byte("this is my file data")

				for _, password := range []string{"", "password"} {

					upload := FileUpload{
						ExpiresIn:        time.Duration(1) * time.Hour,
						Filename:         "test.txt",
						Password:         password,
						AllowedDownloads: 1,
						MemoryOnly:       useMemoOnly,
						FStream:          bytes.NewReader(data),
					}

					fileId, err := filemap.SaveFile(upload)

					assert.Nil(err, "using password of `%s`", password)
					assert.NotNil(fileId)

					var buf2 bytes.Buffer
					assert.Nil(filemap.ReadFile(&buf2, *fileId, password))
					assert.Equal(data, buf2.Bytes(), "using password of `%s`", password)

					assert.NotNil(filemap.ReadFile(&buf2, *fileId, password), "file should be expired")
				}

				filemap.metadataStore.ClearExpiredFiles()
			})

			t.Run("test read & write using threads", func(t *testing.T) {

				assert := assert.New(t)
				tempDir := t.TempDir()

				filemap := NewStorageHandler(tempDir, store, bcrypt.MinCost)

				data := []byte("this is my file data")

				for _, password := range []string{"", "password"} {

					// Simulate many people trying to download the file.
					// successN number of people should be able to download it.
					// failN number of people should not be able to download it.
					n := 500
					successN := n / 2
					failN := n - successN

					// upload the file with the set limits
					upload := FileUpload{
						ExpiresIn:        time.Duration(1) * time.Hour,
						Filename:         "test.txt",
						Password:         password,
						AllowedDownloads: successN, // limit number of downloads
						FStream:          bytes.NewReader(data),
					}

					fileIdPtr, err := filemap.SaveFile(upload)

					assert.Nil(err, "using password of `%s`", password)
					assert.NotNil(fileIdPtr)

					var fileId FileID = *fileIdPtr
					var sCount atomic.Int32
					var fCount atomic.Int32
					var wg sync.WaitGroup
					wg.Add(n)

					// simulate n concurrent downloads
					for range n {

						go func() {
							var buf2 bytes.Buffer

							err := filemap.ReadFile(&buf2, fileId, password)

							if err != nil {
								// failed to read the file, count and assert the failure
								fCount.Add(1)
								assert.EqualValues(ErrFileExpired, err, "expected an expirey error")
							} else {
								// read the file, count and assert it
								sCount.Add(1)
								assert.Equal(data, buf2.Bytes(), "using password of `%s`", password)
							}

							wg.Done()
						}()
					}
					wg.Wait()
					assert.Equal(int32(failN), fCount.Load(), "expected this many download fails")
					assert.Equal(int32(successN), sCount.Load(), "expected this many downloads sucesses")
				}

				filemap.metadataStore.ClearExpiredFiles()
			})
		}
	}
}

// --- Memory only files ---

func TestMemoryOnlyFiles(t *testing.T) {

	filePath := func(t *testing.T, f *StorageHandler, id FileID) string {
		t.Helper()
		var path string
		_, err := f.metadataStore.WithFile(id, func(file *KiFile) error {
			path = file.FilePath
			return nil
		})
		assert.NoError(t, err)
		return path
	}

	for _, memoryOnly := range []bool{true, false} {

		t.Run(fmt.Sprintf("full upload memoryOnly=%v goes in the right folder", memoryOnly), func(t *testing.T) {
			f := newTestFileStore(t)
			upload := validUpload()
			upload.MemoryOnly = memoryOnly
			upload.FStream = bytes.NewReader([]byte("hello"))

			id, err := f.SaveFile(upload)
			assert.NoError(t, err)

			want := f.FileDir
			if memoryOnly {
				want = f.memoryFileDir()
			}
			assert.Equal(t, want, filepath.Dir(filePath(t, &f, *id)))
		})

		t.Run(fmt.Sprintf("chunked upload memoryOnly=%v goes in the right folder", memoryOnly), func(t *testing.T) {
			f := newTestFileStore(t)
			upload := validUpload()
			upload.MemoryOnly = memoryOnly

			sid, err := f.CreateUploadSession(upload, "alice")
			assert.NoError(t, err)

			session, _ := f.uploadSessionStore.get(sid)

			want := f.FileDir
			if memoryOnly {
				want = f.memoryFileDir()
			}
			assert.Equal(t, want, filepath.Dir(session.TempFile.Name()))
		})
	}

	t.Run("ClearMemoryFiles only deletes memory only files", func(t *testing.T) {
		f := newTestFileStore(t)

		memUpload := validUpload()
		memUpload.MemoryOnly = true
		memUpload.FStream = bytes.NewReader([]byte("memory"))
		memID, err := f.SaveFile(memUpload)
		assert.NoError(t, err)

		diskUpload := validUpload()
		diskUpload.FStream = bytes.NewReader([]byte("disk"))
		diskID, err := f.SaveFile(diskUpload)
		assert.NoError(t, err)

		memPath := filePath(t, &f, *memID)
		diskPath := filePath(t, &f, *diskID)

		assert.NoError(t, f.ClearMemoryFiles())

		_, err = os.Stat(memPath)
		assert.True(t, os.IsNotExist(err), "memory only file should be deleted")

		_, err = os.Stat(diskPath)
		assert.NoError(t, err, "disk file should still exist")
	})

	t.Run("ClearMemoryFiles is fine when the folder does not exist", func(t *testing.T) {
		f := newTestFileStore(t)
		assert.NoError(t, f.ClearMemoryFiles())
	})
}
