package storage

import "fmt"

var (
	errNilPtr     = fmt.Errorf("got nil pointer")
	ErrFileExists = fmt.Errorf("the given id exists")
)

// FileStore handles saving the uploaded file information.
type FileStore interface {

	// StoreFileMetadata creates a new FileID and saves the file with it.
	// This clones the given KiFile.
	StoreFile(file *KiFile) (FileID, error)

	// StoreFileEx calls the given gen to get a new FileID from the user.
	// The FileID is then used to store the file, if gen returns an error the file store is aborted.
	// This clones the given KiFile.
	StoreFileEx(file *KiFile, gen func(id *FileID) error) (FileID, error)

	// GetFileMetadata reads a copy of the file's metadata for the given key.
	// Never returns an expired file's metadata.
	GetFileMetadata(id FileID) (*KiMetadata, bool)

	// WithFile gets a write lock on the file allowing for mutation in the store.
	// Any change to the file in this method will be persisted into the store unless he mutate function returns an error.
	// The final state of the file will be returned as a copy, unless there was an error.
	// Mutate may get an expired file, and it should check for that case.
	WithFile(id FileID, mutate func(file *KiFile) error) (*KiFile, error)

	// ClearExpiredFiles deletes any files from the store and disk which have been expired.
	ClearExpiredFiles()
}
