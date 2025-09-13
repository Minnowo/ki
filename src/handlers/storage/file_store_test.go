package storage

import (
	"fmt"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var nonExistID = FileID([]byte("this_id_does_not_exists"))

func getStores(t *testing.T) []FileStore {

	tempDir := t.TempDir()

	bstore, err := NewBBoltFileStore(path.Join(tempDir, t.Name()))
	assert.Nil(t, err, "should be able to create database")

	mstore, err := NewMixedFileStore(path.Join(tempDir, t.Name()+"mixed"))
	assert.Nil(t, err, "should be able to create mixed file store")

	stores := make([]FileStore, 3)
	stores[0] = NewMemoryFileStore()
	stores[1] = bstore
	stores[2] = mstore

	return stores
}

func TestFileStore_StoreFile(t *testing.T) {

	for _, useMemoryOnly := range []bool{false, true} {

		for _, store := range getStores(t) {

			// Create a sample KiFile
			file := &KiFile{
				KiMetadata: KiMetadata{
					Sha512Hash:       make([]byte, 64),
					Sha256Hash:       make([]byte, 32),
					Sha1Hash:         make([]byte, 20),
					Md5Hash:          make([]byte, 16),
					Size:             1024,
					Expires:          time.Now().Add(time.Hour),
					AllowedDownloads: 10,
					Downloads:        0,
					ActiveDownloads:  0,
					Name:             "testfile.txt",
					UserSetPassword:  false,
					MemoryOnly:       useMemoryOnly,
				},
				FilePath:         "/tmp/testfile",
				Key:              make([]byte, 32),
				UserPasswordHash: nil,
				WasCleaned:       false,
				KeySize:          32,
			}

			t.Run("successful store", func(t *testing.T) {
				id, err := store.StoreFile(file.Clone())
				assert.NoError(t, err)
				assert.NotEmpty(t, id)

				// Verify metadata can be retrieved
				metadata, exists := store.GetFileMetadata(id)
				assert.True(t, exists)
				assert.Equal(t, file.Name, metadata.Name)
				assert.Equal(t, file.Size, metadata.Size)
				assert.False(t, metadata.IsExpired())
			})

			t.Run("nil file", func(t *testing.T) {
				id, err := store.StoreFile(nil)
				assert.Error(t, err)
				assert.Empty(t, id)
			})
		}
	}
}

func TestFileStore_GetFileMetadata(t *testing.T) {

	for _, useMemoryOnly := range []bool{false, true} {
		for _, store := range getStores(t) {

			// Store a file
			file := &KiFile{
				KiMetadata: KiMetadata{
					Sha512Hash:       make([]byte, 64),
					Sha256Hash:       make([]byte, 32),
					Sha1Hash:         make([]byte, 20),
					Md5Hash:          make([]byte, 16),
					Size:             1024,
					Expires:          time.Now().Add(-time.Hour), // Expired
					AllowedDownloads: 10,
					Downloads:        10,
					ActiveDownloads:  0,
					Name:             "expiredfile.txt",
					UserSetPassword:  false,
					MemoryOnly:       useMemoryOnly,
				},
				FilePath:   "/tmp/expiredfile",
				Key:        make([]byte, 32),
				WasCleaned: false,
				KeySize:    32,
			}

			t.Run("get non-expired file", func(t *testing.T) {

				// Store a non-expired file
				nonExpiredFile := file.Clone()
				nonExpiredFile.Expires = time.Now().Add(time.Hour)
				nonExpiredFile.Downloads = 0

				id, err := store.StoreFile(nonExpiredFile)
				assert.NoError(t, err)

				metadata, exists := store.GetFileMetadata(id)
				assert.True(t, exists)
				assert.Equal(t, nonExpiredFile.Name, metadata.Name)
				assert.False(t, metadata.IsExpired())
			})

			t.Run("get expired file", func(t *testing.T) {

				file := file.Clone()

				id, err := store.StoreFile(file)
				assert.NoError(t, err)

				metadata, exists := store.GetFileMetadata(id)
				assert.False(t, exists)
				assert.Nil(t, metadata)
			})

			t.Run("get non-existent file", func(t *testing.T) {
				metadata, exists := store.GetFileMetadata(nonExistID)
				assert.False(t, exists)
				assert.Nil(t, metadata)
			})
		}
	}
}

func TestFileStore_WithFile(t *testing.T) {

	for _, useMemoryOnly := range []bool{false, true} {
		for _, store := range getStores(t) {

			// Store a file
			file := &KiFile{
				KiMetadata: KiMetadata{
					Sha512Hash:       make([]byte, 64),
					Sha256Hash:       make([]byte, 32),
					Sha1Hash:         make([]byte, 20),
					Md5Hash:          make([]byte, 16),
					Size:             1024,
					Expires:          time.Now().Add(time.Hour),
					AllowedDownloads: 10,
					Downloads:        0,
					ActiveDownloads:  0,
					Name:             "mutablefile.txt",
					UserSetPassword:  false,
					MemoryOnly:       useMemoryOnly,
				},
				FilePath:   "/tmp/mutablefile",
				Key:        make([]byte, 32),
				WasCleaned: false,
				KeySize:    32,
			}

			t.Run("successful mutation", func(t *testing.T) {

				file := file.Clone()

				id, err := store.StoreFile(file)
				assert.NoError(t, err)

				newName := "updatedfile.txt"
				updatedFile, err := store.WithFile(id, func(f *KiFile) error {
					f.Name = newName
					return nil
				})
				assert.NoError(t, err)
				assert.Equal(t, newName, updatedFile.Name)

				// Verify persisted change
				metadata, exists := store.GetFileMetadata(id)
				assert.True(t, exists)
				assert.Equal(t, newName, metadata.Name)
			})

			t.Run("mutation with error", func(t *testing.T) {

				file := file.Clone()

				id, err := store.StoreFile(file)
				assert.NoError(t, err)

				originalName := file.Name
				updatedFile, err := store.WithFile(id, func(f *KiFile) error {
					f.Name = "failedupdate.txt"
					return fmt.Errorf("mutation failed")
				})
				assert.Error(t, err)
				assert.Nil(t, updatedFile)

				// Verify no changes persisted
				metadata, exists := store.GetFileMetadata(id)
				assert.True(t, exists)
				assert.Equal(t, originalName, metadata.Name)
			})

			t.Run("expired file", func(t *testing.T) {

				file := file.Clone()
				file.Expires = time.Now().Add(-time.Hour)

				id, err := store.StoreFile(file)
				assert.NoError(t, err)

				updatedFile, err := store.WithFile(id, func(f *KiFile) error {
					if f.IsExpired() {
						return fmt.Errorf("file is expired")
					}
					f.Name = "shouldnotpersist.txt"
					return nil
				})
				assert.Error(t, err)
				assert.Nil(t, updatedFile)
			})

			t.Run("non-existent file", func(t *testing.T) {

				updatedFile, err := store.WithFile(nonExistID, func(f *KiFile) error {
					return nil
				})
				assert.Error(t, err)
				assert.Nil(t, updatedFile)
			})
		}
	}
}

func TestFileStore_ClearExpiredFiles(t *testing.T) {
	for _, useMemoryOnly := range []bool{false, true} {
		for _, store := range getStores(t) {

			// Store a non-expired file
			nonExpiredFile := &KiFile{
				KiMetadata: KiMetadata{
					Sha512Hash:       make([]byte, 64),
					Sha256Hash:       make([]byte, 32),
					Sha1Hash:         make([]byte, 20),
					Md5Hash:          make([]byte, 16),
					Size:             1024,
					Expires:          time.Now().Add(time.Hour),
					AllowedDownloads: 10,
					Downloads:        0,
					ActiveDownloads:  0,
					Name:             "nonexpiredfile.txt",
					UserSetPassword:  false,
					MemoryOnly:       useMemoryOnly,
				},
				FilePath:   "/tmp/nonexpiredfile",
				Key:        make([]byte, 32),
				WasCleaned: false,
				KeySize:    32,
			}

			// Store an expired file
			expiredFile := &KiFile{
				KiMetadata: KiMetadata{
					Sha512Hash:       make([]byte, 64),
					Sha256Hash:       make([]byte, 32),
					Sha1Hash:         make([]byte, 20),
					Md5Hash:          make([]byte, 16),
					Size:             1024,
					Expires:          time.Now().Add(-time.Hour),
					AllowedDownloads: 10,
					Downloads:        10,
					ActiveDownloads:  0,
					Name:             "expiredfile.txt",
					UserSetPassword:  false,
				},
				FilePath:   "/tmp/expiredfile",
				Key:        make([]byte, 32),
				WasCleaned: false,
				KeySize:    32,
			}

			t.Run("clear expired files", func(t *testing.T) {

				expiredFile := expiredFile.Clone()

				expiredID, err := store.StoreFile(expiredFile)
				assert.NoError(t, err)

				nonExpiredFile := nonExpiredFile.Clone()

				nonExpiredID, err := store.StoreFile(nonExpiredFile)
				assert.NoError(t, err)

				store.ClearExpiredFiles()

				// Non-expired file should still exist
				metadata, exists := store.GetFileMetadata(nonExpiredID)
				assert.True(t, exists)
				assert.Equal(t, nonExpiredFile.Name, metadata.Name)

				// Expired file should be gone
				metadata, exists = store.GetFileMetadata(expiredID)
				assert.False(t, exists)
				assert.Nil(t, metadata)
			})
		}
	}
}
