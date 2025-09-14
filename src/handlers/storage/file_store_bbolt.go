package storage

import (
	"crypto/rand"
	"fmt"
	"ki/src/handlers/crypto"
	"time"

	"github.com/minnowo/log4zero"
	bolt "go.etcd.io/bbolt"
)

var (
	bBucketFiles = []byte("file_data")
)

var (
	errBucketNotExist = fmt.Errorf("bucket does not exist")
	errKeyExists      = fmt.Errorf("key exists")
)

var (
	boltLog = log4zero.Get("BBoltFileStore")
)

type BBoltFileStore struct {
	db        *bolt.DB
	encrypt   bool
	keySize   crypto.AESKeySize
	masterKey string
}

func NewBBoltFileStore(path string) (*BBoltFileStore, error) {

	boltLog.Debug().Str("path", path).Msg("opening bolt database")

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

		// encrypt all metadata. except for the file id.
		// TODO: also encrypt the file id. Or just the entire database.
		// TODO: configure this somewhere.
		encrypt:   false,
		keySize:   crypto.AES256,
		masterKey: "super secret key",
	}

	return fstore, nil
}

func (s *BBoltFileStore) StoreFileEx(file *KiFile, gen func(*FileID) error) (FileID, error) {

	if file == nil {
		return FileID{}, errNilPtr
	}

	var id FileID

	err := s.db.Update(func(tx *bolt.Tx) error {

		b := tx.Bucket(bBucketFiles)

		if b == nil {
			return errBucketNotExist
		}

		for {
			if err := gen(&id); err != nil {
				return err
			}

			v := b.Get(id[:])

			if v == nil {
				break
			}
		}

		data, err := file.ToBinary()

		if err != nil {
			return err
		}

		boltLog.Debug().Str("name", file.Name).Msg("storing file")

		if s.encrypt {

			encrypted, err := crypto.EncryptBytes(s.keySize, s.masterKey, id[:], data)

			if err != nil {
				return err
			}

			data = encrypted
		}

		return b.Put(id[:], data)
	})

	if err != nil {
		return FileID{}, err
	}

	return id, nil
}

func (s *BBoltFileStore) StoreFile(file *KiFile) (FileID, error) {
	return s.StoreFileEx(file, func(id *FileID) error {
		_, err := rand.Read(id[:])
		return err
	})
}

func (s *BBoltFileStore) GetFileMetadata(id FileID) (*KiMetadata, bool) {

	var file KiFile

	err := s.db.View(func(tx *bolt.Tx) error {

		b := tx.Bucket(bBucketFiles)

		if b == nil {
			return errBucketNotExist
		}

		data := b.Get(id[:])

		if s.encrypt {

			decrypted, err := crypto.DecryptBytes(s.keySize, s.masterKey, id[:], data)

			if err != nil {
				return err
			}

			data = decrypted
		}

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

		if s.encrypt {

			decrypted, err := crypto.DecryptBytes(s.keySize, s.masterKey, id[:], data)

			if err != nil {
				return err
			}

			data = decrypted
		}

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

		if s.encrypt {

			encrypted, err := crypto.EncryptBytes(s.keySize, s.masterKey, id[:], data)

			if err != nil {
				return err
			}

			data = encrypted
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

			if s.encrypt {

				decrypted, err := crypto.DecryptBytes(s.keySize, s.masterKey, k, v)

				if err != nil {
					return err
				}

				v = decrypted
			}

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
