package storage

import (
	"crypto/rand"
	"sync"

	"github.com/rs/zerolog/log"
)

// FileMetadataStore handles saving the uploaded file information.
// Depending on the implementation of the store, modifying the data of the file pointer might also update the data in the store.
// To ensure the data is up-to-date in the store, call SetFileMetadata after modifying the data.
type FileMetadataStore interface {

	// GetFileMetadata reads the file metadata for the given key.
	GetFileMetadata(id FileID) (*SafeFileEx, bool)

	// SetFileMetadata create or replace some metadata with the given key.
	SetFileMetadata(id FileID, file *SafeFileEx)

	// StoreFileMetadata creates a new FileID and saves associates the metadata with it.
	StoreFileMetadata(file *SafeFileEx) FileID

	// ClearExpiredFiles deletes any files from the store and disk which have been expired.
	ClearExpiredFiles()
}

// MemoryFileMetadataStore a simple file store which uses a hashmap.
type MemoryFileMetadataStore struct {
	sync.RWMutex
	files map[FileID]*SafeFileEx
}

func NewMemoryFileMetadataStore() *MemoryFileMetadataStore {
	return &MemoryFileMetadataStore{
		files: make(map[FileID]*SafeFileEx),
	}
}

func (s *MemoryFileMetadataStore) GetFileMetadata(id FileID) (*SafeFileEx, bool) {

	s.RLock()
	file, ok := s.files[id]
	s.RUnlock()

	if !ok {
		return nil, false
	}

	return file, true
}

func (s *MemoryFileMetadataStore) StoreFileMetadata(file *SafeFileEx) FileID {

	s.Lock()
	defer s.Unlock()

	var fileId FileID
	for {
		rand.Read(fileId[:])

		if _, ok := s.files[fileId]; !ok {
			break
		}
	}

	s.files[fileId] = file

	return fileId
}

func (s *MemoryFileMetadataStore) SetFileMetadata(id FileID, file *SafeFileEx) {

	s.Lock()
	defer s.Unlock()

	s.files[id] = file
}

func (s *MemoryFileMetadataStore) ClearExpiredFiles() {

	s.RLock()

	expired := make([]FileID, 0, len(s.files))

	for key, value := range s.files {

		if value.CleanIfExpired() && value.Clean() {
			expired = append(expired, key)
		}
	}

	s.RUnlock()

	if len(expired) <= 0 {
		return
	}

	log.Info().Int("count", len(expired)).Msg("removing expired files")

	s.Lock()
	defer s.Unlock()

	for _, key := range expired {

		_, ok := s.files[key]

		if ok {
			delete(s.files, key)
		}
	}

}
