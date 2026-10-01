package storage

import (
	"crypto/rand"
	"fmt"

	"github.com/minnowo/log4zero"
)

var (
	errInvalidFileID = fmt.Errorf("the generated file id is invalid")
)

var (
	mixedLog = log4zero.Get("MixedFileStore")
)

// MixedFileStore picks between a disk and memory store based off of the uploaded file metadata.
// FileIDs are used to determine which store the file was placed in.
// See the IsMemoryOnly method which determines if a FileID is for a memoryStore or a diskStore.
type MixedFileStore struct {
	memoryStore *MemoryFileStore
	diskStore   *BBoltFileStore
}

func NewMixedFileStore(path string) (*MixedFileStore, error) {

	diskStore, err := NewBBoltFileStore(path)

	if err != nil {
		return nil, err
	}

	memStore := NewMemoryFileStore()

	return &MixedFileStore{
		memoryStore: memStore,
		diskStore:   diskStore,
	}, nil
}

// IsMemoryOnly determines if the given FileID belongs to the memoryStore or the diskStore.
// If the FileID[0] is even, it belongs in the memoryStore, otherwise it belongs in the diskStore.
func (s *MixedFileStore) IsMemoryOnly(id *FileID) bool {
	return id[0]%2 == 0
}

func (s *MixedFileStore) StoreFileEx(file *KiFile, gen func(*FileID) error) (FileID, error) {

	if file == nil || gen == nil {
		return FileID{}, errNilPtr
	}

	var store FileStore

	if file.MemoryOnly {
		store = s.memoryStore
	} else {
		store = s.diskStore
	}

	mixedLog.Debug().
		Bool("mem", file.MemoryOnly).
		Str("name", file.Name).
		Msg("storing file")

	return store.StoreFileEx(file, func(id *FileID) error {

		if err := gen(id); err != nil {
			return err
		}

		if file.MemoryOnly {
			if !s.IsMemoryOnly(id) {
				return errInvalidFileID
			}
		} else {
			if s.IsMemoryOnly(id) {
				return errInvalidFileID
			}
		}

		return nil
	})
}

func (s *MixedFileStore) StoreFile(file *KiFile) (FileID, error) {

	if file == nil {
		return FileID{}, errNilPtr
	}

	return s.StoreFileEx(file, func(id *FileID) error {

		if _, err := rand.Read(id[:]); err != nil {
			return err
		}

		if file.MemoryOnly {
			if !s.IsMemoryOnly(id) {
				id[0] += 1
			}
		} else {
			if s.IsMemoryOnly(id) {
				id[0] += 1
			}
		}

		return nil
	})
}

func (s *MixedFileStore) GetFile(id FileID) (*KiFile, bool) {

	if s.IsMemoryOnly(&id) {
		return s.memoryStore.GetFile(id)
	}

	return s.diskStore.GetFile(id)
}

func (s *MixedFileStore) GetFileMetadata(id FileID) (*KiMetadata, bool) {

	if s.IsMemoryOnly(&id) {
		return s.memoryStore.GetFileMetadata(id)
	}

	return s.diskStore.GetFileMetadata(id)
}

func (s *MixedFileStore) WithFile(id FileID, mutate func(file *KiFile) error) (*KiFile, error) {

	if s.IsMemoryOnly(&id) {
		return s.memoryStore.WithFile(id, mutate)
	}

	return s.diskStore.WithFile(id, mutate)
}

func (s *MixedFileStore) ClearExpiredFiles() {
	s.memoryStore.ClearExpiredFiles()
	s.diskStore.ClearExpiredFiles()
}
