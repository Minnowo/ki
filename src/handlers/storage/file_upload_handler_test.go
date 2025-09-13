package storage

import (
	"bytes"
	"ki/src/config"
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

func TestFileMap(t *testing.T) {

	for _, store := range getStores(t) {

		for _, useMemoOnly := range []bool{false, true} {

			t.Run("test read & write", func(t *testing.T) {

				assert := assert.New(t)
				tempDir := t.TempDir()

				filemap := NewFileStore(tempDir, store, bcrypt.MinCost)

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

				filemap := NewFileStore(tempDir, store, bcrypt.MinCost)

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
					assert.Equal(int32(failN), fCount.Load(), "expected this many fails")
					assert.Equal(int32(successN), sCount.Load(), "expected this many fails")
				}

				filemap.metadataStore.ClearExpiredFiles()
			})
		}
	}
}
