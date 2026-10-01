package storage

import (
	"crypto/rand"
	"fmt"
	"ki/src/config"
	"ki/src/handlers/crypto"
	"time"

	"github.com/minnowo/log4zero"
	"github.com/rs/zerolog/log"
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
		encrypt:   true,
		keySize:   crypto.AES256,
		masterKey: config.GetMasterSecret(),
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

func (s *BBoltFileStore) GetFile(id FileID) (*KiFile, bool) {

	var file KiFile

	err := s.db.View(func(tx *bolt.Tx) error {

		b := tx.Bucket(bBucketFiles)

		if b == nil {
			return errBucketNotExist
		}

		data := b.Get(id[:])

		if data == nil {
			return ErrFileNotFound
		}

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

	return &file, true
}

func (s *BBoltFileStore) GetFileMetadata(id FileID) (*KiMetadata, bool) {

	file, ok := s.GetFile(id)

	if !ok || file.IsExpired() {
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

		if data == nil {
			return ErrFileNotFound
		}

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

// ClearExpiredFiles finds expired files with a read-only transaction, cleans them, and then deletes them from the store.
// Only the final delete holds the write lock, since decrypting every file is slow.
// An expired file can never become unexpired, so nothing needs to be checked again between the two transactions.
func (s *BBoltFileStore) ClearExpiredFiles() {

	expired := make(map[FileID]*KiFile)

	err := s.db.View(func(tx *bolt.Tx) error {

		b := tx.Bucket(bBucketFiles)

		if b == nil {
			return errBucketNotExist
		}

		return b.ForEach(func(k []byte, v []byte) error {

			var file KiFile

			if s.encrypt {

				decrypted, err := crypto.DecryptBytes(s.keySize, s.masterKey, k, v)

				if err != nil {

					log.Warn().Hex("key", k).Err(err).Msg("unable to decrypt file")

					return nil
				}

				v = decrypted
			}

			if err := file.FromBinary(v); err != nil {

				log.Warn().Hex("key", k).Err(err).Msg("unable to convert the file from binary")

				return nil
			}

			if file.IsExpired() {

				// k is only valid during the transaction, so it is copied into the FileID
				var id FileID
				copy(id[:], k)

				expired[id] = &file
			}

			return nil
		})
	})

	if err != nil {
		boltLog.Error().Err(err).Msg("failed to scan for expired files")
		return
	}

	var cleaned []FileID

	for id, file := range expired {

		// files which fail to be removed from disk stay in the store, so they are retried next time
		if file.Clean() {
			cleaned = append(cleaned, id)
		}
	}

	if len(cleaned) == 0 {
		return
	}

	err = s.db.Update(func(tx *bolt.Tx) error {

		b := tx.Bucket(bBucketFiles)

		if b == nil {
			return errBucketNotExist
		}

		for _, id := range cleaned {
			if err := b.Delete(id[:]); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		boltLog.Error().Err(err).Msg("failed to delete expired files")
		return
	}

	boltLog.Info().Int("count", len(cleaned)).Msg("removed expired files")
}
