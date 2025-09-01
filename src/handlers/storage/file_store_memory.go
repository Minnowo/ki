package storage

import (
	"crypto/rand"
	"sync"

	"github.com/rs/zerolog/log"
)

type FileLock struct {
	File KiFile
	sync.RWMutex
}

func (f *FileLock) DoExpire() {
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

func (s *MemoryFileMetadataStore) WithFile(id FileID, mutate func(file KiFile) error) (KiFile, error) {

	s.RLock()
	file, ok := s.files[id]
	s.RUnlock()

	if !ok {
		return nil, ErrFileNotFound
	}

	file.Lock()
	defer file.Unlock()

	newFile := file.File.Clone()

	if err := mutate(newFile); err != nil {
		return nil, err
	}

	file.File = newFile.Clone()

	// we can expire the copy here, but the caller expects to get a view of the file from inside their mutate method.
	if file.File.IsExpired() {

		if file.File.Clean() {
			s.Lock()
			delete(s.files, id)
			s.Unlock()
		}
	}

	return newFile, nil
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

func (s *MemoryFileMetadataStore) StoreFileCopy(file KiFile) FileID {
	return s.StoreFile(file.Clone())
}

func (s *MemoryFileMetadataStore) StoreFile(file KiFile) FileID {

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

	return fileId
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
