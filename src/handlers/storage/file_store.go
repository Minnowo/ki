package storage

import (
	"crypto/rand"
	"sync"

	"github.com/rs/zerolog/log"
)

// FileStore handles saving the uploaded file information.
// Depending on the implementation of the store, modifying the data of the file pointer might also update the data in the store.
// To ensure the data is up-to-date in the store, call SetFileMetadata after modifying the data.
type FileStore interface {

	// GetFileMetadata reads a copy of the file's metadata for the given key.
	// Never returns an expired file's metadata.
	GetFileMetadata(id FileID) (*SafeFile, bool)

	// GetFile reads a copy of the file for the given key.
	// Never returns an expired file.
	GetFile(id FileID) (*SafeFileEx, bool)

	// SetFileMetadata create or replace some metadata with the given key.
	SetFile(id FileID, file *SafeFileEx)

	// StoreFileMetadata creates a new FileID and saves associates the metadata with it.
	StoreFile(file *SafeFileEx) FileID

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

func (s *MemoryFileMetadataStore) GetFile(id FileID) (*SafeFileEx, bool) {

	s.RLock()
	file, ok := s.files[id]
	s.RUnlock()

	if !ok {
		return nil, false
	}

	if file.IsExpired() {

		file.Clean()

		return nil, false
	}

	return file, true
}

func (s *MemoryFileMetadataStore) GetFileMetadata(id FileID) (*SafeFile, bool) {

	file, ok := s.GetFile(id)

	if !ok {
		return nil, false
	}

	return file.SafeFile.Copy(), true
}

func (s *MemoryFileMetadataStore) StoreFile(file *SafeFileEx) FileID {

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

func (s *MemoryFileMetadataStore) SetFile(id FileID, file *SafeFileEx) {

	s.Lock()
	defer s.Unlock()

	s.files[id] = file
}

func (s *MemoryFileMetadataStore) ClearExpiredFiles() {

	s.Lock()
	defer s.Unlock()

	expired := 0

	for key, value := range s.files {

		if value.IsExpired() && value.Clean() {

			delete(s.files, key)

			expired++
		}
	}

	if expired > 0 {
		log.Info().Int("count", expired).Msg("removed expired files")
	}
}
