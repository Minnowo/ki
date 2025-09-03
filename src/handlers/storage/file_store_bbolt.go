package storage

import (
	"crypto/rand"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

var (
	bBucketFiles = []byte("file_data")
)

var (
	errBucketNotExist = fmt.Errorf("bucket does not exist")
	errKeyExists      = fmt.Errorf("key exists")
)

type BBoltFileStore struct {
	db *bolt.DB
}

func NewBBoltFileStore(path string) (*BBoltFileStore, error) {

	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: time.Second * 5})

	if err != nil {
		return nil, err
	}

	db.Update(func(tx *bolt.Tx) error {

		_, err := tx.CreateBucketIfNotExists(bBucketFiles)

		return err
	})

	fstore := &BBoltFileStore{
		db: db,
	}

	return fstore, nil
}

func (s *BBoltFileStore) StoreFileCopy(file *KiFile) (FileID, error) {
	return s.StoreFile(file)
}

func (s *BBoltFileStore) StoreFile(file *KiFile) (FileID, error) {

	if file == nil {
		return FileID{}, errNilPtr
	}

	var fileId FileID

	err := s.db.Update(func(tx *bolt.Tx) error {

		b := tx.Bucket(bBucketFiles)

		if b == nil {
			return errBucketNotExist
		}

		for {
			rand.Read(fileId[:])

			v := b.Get(fileId[:])

			if v == nil {
				break
			}
		}

		data, err := file.ToBinary()

		if err != nil {
			return err
		}

		return b.Put(fileId[:], data)
	})

	if err != nil {
		return FileID{}, err
	}

	return fileId, nil
}

func (s *BBoltFileStore) GetFileMetadata(id FileID) (*KiMetadata, bool) {

	var file KiFile

	err := s.db.View(func(tx *bolt.Tx) error {

		b := tx.Bucket(bBucketFiles)

		if b == nil {
			return errBucketNotExist
		}

		data := b.Get(id[:])

		return file.FromBinary(data)
	})

	if err != nil {
		return nil, false
	}

	if file.IsExpired() {
		return nil, false
	}

	return file.Metadata(), true
}

func (s *BBoltFileStore) WithFile(id FileID, mutate func(file *KiFile) error) (*KiFile, error) {

	var file KiFile

	err := s.db.Update(func(tx *bolt.Tx) error {

		b := tx.Bucket(bBucketFiles)

		if b == nil {
			return errBucketNotExist
		}

		data := b.Get(id[:])

		if err := file.FromBinary(data); err != nil {
			return err
		}

		if err := mutate(&file); err != nil {
			return err
		}

		data, err := file.ToBinary()

		if err != nil {
			return err
		}

		return b.Put(id[:], data)
	})

	if err != nil {
		return nil, err
	}

	return &file, nil
}

func (s *BBoltFileStore) ClearExpiredFiles() {

	s.db.Update(func(tx *bolt.Tx) error {

		b := tx.Bucket(bBucketFiles)

		if b == nil {
			return errBucketNotExist
		}

		return b.ForEach(func(k []byte, v []byte) error {

			var file KiFile

			if err := file.FromBinary(v); err != nil {
				return err
			}

			if file.IsExpired() && file.Clean() {

				if err := b.Delete(k); err != nil {
					return err
				}
			}

			return nil
		})
	})
}
