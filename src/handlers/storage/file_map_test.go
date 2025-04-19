package storage

import (
	"bytes"
	"ki/src/handlers/crypto"
	"ki/src/logging"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

func TestFileMap(t *testing.T) {

	logging.Init()

	t.Run("test read & write", func(t *testing.T) {

		assert := assert.New(t)
		tempDir := t.TempDir()

		filemap := NewFileMap(crypto.AES256, bcrypt.MinCost)
		filemap.TempDir = tempDir

		data := []byte("this is my file data")

		for _, password := range []string{"", "password"} {

			upload := SafeFileUpload{
				ExpiresIn:        time.Duration(1) * time.Hour,
				Filename:         "test.txt",
				Password:         password,
				AllowedDownloads: 1,
			}

			fileId, err := filemap.SaveFile(upload, bytes.NewReader(data), nil)

			assert.Nil(err, "using password of %s", password)
			assert.NotNil(fileId)

			var buf2 bytes.Buffer
			assert.Nil(filemap.ReadFile(&buf2, *fileId, password))
			assert.Equal(data, buf2.Bytes(), "using password of %s", password)

			assert.NotNil(filemap.ReadFile(&buf2, *fileId, password), "file should be expired")
		}

		filemap.RemoveExpired()
	})

	t.Run("test read & write using threads", func(t *testing.T) {

		assert := assert.New(t)
		tempDir := t.TempDir()

		filemap := NewFileMap(crypto.AES256, bcrypt.MinCost)
		filemap.TempDir = tempDir

		data := []byte("this is my file data")

		for _, password := range []string{"", "password"} {

			// Simulate 10 people trying to download the file.
			// successN number of people should be able to download it.
			// failN number of people should not be able to download it.
			n := 30
			successN := n / 2
			failN := n - successN

			// upload the file with the set limits
			upload := SafeFileUpload{
				ExpiresIn:        time.Duration(1) * time.Hour,
				Filename:         "test.txt",
				Password:         password,
				AllowedDownloads: successN, // limit number of downloads
			}

			fileId, err := filemap.SaveFile(upload, bytes.NewReader(data), nil)

			assert.Nil(err, "using password of %s", password)
			assert.NotNil(fileId)

			var sCount atomic.Int32
			var fCount atomic.Int32
			var wg sync.WaitGroup
			wg.Add(n)

			// simulate n concurrent downloads
			for range n {

				go func() {
					var buf2 bytes.Buffer

					if filemap.ReadFile(&buf2, *fileId, password) != nil {
						// failed to read the file, count it
						fCount.Add(1)
					} else {
						// read the file, count and assert it
						sCount.Add(1)
						assert.Equal(data, buf2.Bytes(), "using password of %s", password)
					}

					wg.Done()
				}()
			}
			wg.Wait()
			assert.Equal(int32(failN), fCount.Load(), "expected this many fails")
			assert.Equal(int32(successN), sCount.Load(), "expected this many fails")
		}

		filemap.RemoveExpired()
	})
}
