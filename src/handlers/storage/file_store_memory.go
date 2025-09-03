package storage

import (
	"crypto/rand"
	"sync"

	"github.com/rs/zerolog/log"
)

type FileLock struct {
	File *KiFile
	sync.RWMutex
}

// MemoryFileMetadataStore a simple file store which uses a hashmap.
// All file metadata is stored in memory.
type MemoryFileMetadataStore struct {
	sync.RWMutex
	files map[FileID]*FileLock
}

func NewMemoryFileMetadataStore() *MemoryFileMetadataStore {
	return &MemoryFileMetadataStore{
		files: make(map[FileID]*FileLock),
	}
}

func (s *MemoryFileMetadataStore) WithFile(id FileID, mutate func(file *KiFile) error) (*KiFile, error) {

	s.RLock()
	file, ok := s.files[id]
	s.RUnlock()

	if !ok {
		return nil, ErrFileNotFound
	}

	file.Lock()
	defer file.Unlock()

	newFile := file.File.Clone()

	err := mutate(newFile)

	if err != nil {
		newFile = nil
	} else {
		file.File = newFile.Clone()
	}

	// We shouldn't aquire the s.Lock() here.
	// Since ClearExpiredFiles might be waiting for the lock we currently have.
	// If we lock s, then we would have a deadlock.
	if file.File.IsExpired() {
		file.File.Clean()
	}

	return newFile, err
}

func (s *MemoryFileMetadataStore) GetFileMetadata(id FileID) (*KiMetadata, bool) {

	s.RLock()
	file, ok := s.files[id]
	s.RUnlock()

	if !ok {
		return nil, false
	}

	file.RLock()
	defer file.RUnlock()

	if file.File.IsExpired() {
		return nil, false
	}

	return file.File.Metadata().Clone(), true
}

func (s *MemoryFileMetadataStore) StoreFileCopy(file *KiFile) (FileID, error) {

	if file == nil {
		return FileID{}, errNilPtr
	}

	return s.StoreFile(file.Clone())
}

func (s *MemoryFileMetadataStore) StoreFile(file *KiFile) (FileID, error) {

	if file == nil {
		return FileID{}, errNilPtr
	}

	s.Lock()
	defer s.Unlock()

	var fileId FileID
	for {
		rand.Read(fileId[:])

		if _, ok := s.files[fileId]; !ok {
			break
		}
	}

	s.files[fileId] = &FileLock{File: file}

	return fileId, nil
}

func (s *MemoryFileMetadataStore) ClearExpiredFiles() {

	s.Lock()
	defer s.Unlock()

	expired := 0

	for key, value := range s.files {

		value.Lock()

		if value.File.IsExpired() && value.File.Clean() {

			delete(s.files, key)

			expired++
		}
		value.Unlock()
	}

	if expired > 0 {
		log.Info().Int("count", expired).Msg("removed expired files")
	}
}
