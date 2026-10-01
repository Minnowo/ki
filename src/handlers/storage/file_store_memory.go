package storage

import (
	"crypto/rand"
	"sync"

	"github.com/minnowo/log4zero"
)

var (
	memLog = log4zero.Get("MemoryFileStore")
)

type fileLock struct {
	File *KiFile
	sync.RWMutex
}

// MemoryFileStore a simple file store which uses a hashmap.
// All file metadata is stored in memory.
type MemoryFileStore struct {
	sync.RWMutex
	files map[FileID]*fileLock
}

func NewMemoryFileStore() *MemoryFileStore {
	return &MemoryFileStore{
		files: make(map[FileID]*fileLock),
	}
}

func (s *MemoryFileStore) WithFile(id FileID, mutate func(file *KiFile) error) (*KiFile, error) {

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

func (s *MemoryFileStore) GetFile(id FileID) (*KiFile, bool) {

	s.RLock()
	file, ok := s.files[id]
	s.RUnlock()

	if !ok {
		return nil, false
	}

	file.RLock()
	defer file.RUnlock()

	return file.File.Clone(), true
}

func (s *MemoryFileStore) GetFileMetadata(id FileID) (*KiMetadata, bool) {

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

func (s *MemoryFileStore) StoreFileEx(file *KiFile, gen func(id *FileID) error) (FileID, error) {

	if file == nil || gen == nil {
		return FileID{}, errNilPtr
	}

	s.Lock()
	defer s.Unlock()

	var fileId FileID
	for {
		if err := gen(&fileId); err != nil {
			return FileID{}, err
		}

		if _, ok := s.files[fileId]; !ok {
			break
		}
	}

	memLog.Debug().Str("name", file.Name).Msg("storing file")

	s.files[fileId] = &fileLock{File: file}

	return fileId, nil
}

func (s *MemoryFileStore) StoreFile(file *KiFile) (FileID, error) {
	return s.StoreFileEx(file, func(id *FileID) error {
		_, err := rand.Read(id[:])
		return err
	})
}

func (s *MemoryFileStore) ClearExpiredFiles() {

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
		memLog.Info().Int("count", expired).Msg("removed expired files")
	}
}
