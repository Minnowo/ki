package storage

// FileStore handles saving the uploaded file information.
type FileStore interface {

	// StoreFileMetadata creates a new FileID and saves the file with it.
	// Depending on the implementation this will also copy the file.
	// This should be used only when you know you're not modifying the given file after the fact.
	StoreFile(file KiFile) FileID

	// StoreFileMetadata creates a new FileID and saves a clone of the file with it.
	StoreFileCopy(file KiFile) FileID

	// GetFileMetadata reads a copy of the file's metadata for the given key.
	// Never returns an expired file's metadata.
	GetFileMetadata(id FileID) (*KiMetadata, bool)

	// WithFile gets a write lock on the file allowing for mutation in the store.
	// Any change to the file in this method will be persisted into the store unless he mutate function returns an error.
	// The final state of the file will be returned as a copy, unless there was an error.
	// Mutate may get an expired file, and it should check for that case.
	WithFile(id FileID, mutate func(file KiFile) error) (KiFile, error)

	// ClearExpiredFiles deletes any files from the store and disk which have been expired.
	ClearExpiredFiles()
}
