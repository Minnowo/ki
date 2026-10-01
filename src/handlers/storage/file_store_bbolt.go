package storage

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
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

	// bBucketMeta holds the settings used to derive the master key, see openMasterKey.
	bBucketMeta = []byte("meta")

	bMetaKdfSalt   = []byte("kdf_salt")
	bMetaKdfRounds = []byte("kdf_rounds")
	bMetaKeyCheck  = []byte("key_check")
)

const (
	// subkey info for file records, and for the value used to check the master secret.
	infoFileRecord = "ki file record"
	infoKeyCheck   = "ki master key check"
)

var (
	errBucketNotExist = fmt.Errorf("bucket does not exist")
	errKeyExists      = fmt.Errorf("key exists")

	ErrWrongMasterSecret = errors.New("the master secret does not match the one this database was created with")
	ErrLegacyDatabase    = errors.New("this database was created by an older version which encrypted files differently, it cannot be read and must be deleted")
)

var (
	boltLog = log4zero.Get("BBoltFileStore")
)

// masterKeyRounds is the number of PBKDF2 rounds used when creating a new database.
// Existing databases keep the rounds they were created with. Tests lower this to stay fast.
var masterKeyRounds = crypto.MASTER_KEY_ROUNDS

type BBoltFileStore struct {
	db      *bolt.DB
	encrypt bool
	keySize crypto.AESKeySize
	masterKey []byte
}

func NewBBoltFileStore(path string) (*BBoltFileStore, error) {
	return newBBoltFileStore(path, config.GetMasterSecret())
}

func newBBoltFileStore(path string, secret string) (*BBoltFileStore, error) {

	boltLog.Debug().Str("path", path).Msg("opening bolt database")

	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: time.Second * 5})

	if err != nil {
		return nil, err
	}

	fstore := &BBoltFileStore{
		db: db,

		// encrypt all metadata. except for the file id.
		// TODO: also encrypt the file id. Or just the entire database.
		// TODO: configure this somewhere.
		encrypt: true,
		keySize: crypto.AES256,
	}

	err = db.Update(func(tx *bolt.Tx) error {

		files, err := tx.CreateBucketIfNotExists(bBucketFiles)

		if err != nil {
			return err
		}

		meta, err := tx.CreateBucketIfNotExists(bBucketMeta)

		if err != nil {
			return err
		}

		fstore.masterKey, err = openMasterKey(meta, files, fstore.keySize, secret)

		return err
	})

	if err != nil {
		db.Close()
		return nil, err
	}

	return fstore, nil
}

// openMasterKey derives the master key from the master secret.
// A new database gets a new random salt.
func openMasterKey(meta *bolt.Bucket, files *bolt.Bucket, kSize crypto.AESKeySize, secret string) ([]byte, error) {

	salt := meta.Get(bMetaKdfSalt)

	if salt == nil {

		// databases from before the meta bucket have files, but no salt
		if k, _ := files.Cursor().First(); k != nil {
			return nil, ErrLegacyDatabase
		}

		return createMasterKey(meta, kSize, secret)
	}

	roundsBytes := meta.Get(bMetaKdfRounds)

	if len(roundsBytes) != 8 {
		return nil, fmt.Errorf("invalid master key rounds in database")
	}

	rounds := int(binary.BigEndian.Uint64(roundsBytes))

	boltLog.Info().Int("rounds", rounds).Msg("deriving master key")

	masterKey, err := crypto.DeriveMasterKey(kSize, secret, salt, rounds)

	if err != nil {
		return nil, err
	}

	_, err = crypto.OpenWithSubkey(masterKey, nil, infoKeyCheck, meta.Get(bMetaKeyCheck))

	if  err != nil {
		return nil, ErrWrongMasterSecret
	}

	return masterKey, nil
}

func createMasterKey(meta *bolt.Bucket, kSize crypto.AESKeySize, secret string) ([]byte, error) {

	salt := make([]byte, 32)

	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}

	boltLog.Info().Int("rounds", masterKeyRounds).Msg("creating master key for new database")

	masterKey, err := crypto.DeriveMasterKey(kSize, secret, salt, masterKeyRounds)

	if err != nil {
		return nil, err
	}

	check, err := crypto.SealWithSubkey(masterKey, nil, infoKeyCheck, []byte("ki"))

	if err != nil {
		return nil, err
	}

	rounds := binary.BigEndian.AppendUint64(nil, uint64(masterKeyRounds))

	if err := meta.Put(bMetaKdfSalt, salt); err != nil {
		return nil, err
	}

	if err := meta.Put(bMetaKdfRounds, rounds); err != nil {
		return nil, err
	}

	if err := meta.Put(bMetaKeyCheck, check); err != nil {
		return nil, err
	}

	return masterKey, nil
}

// Close closes the database.
func (s *BBoltFileStore) Close() error {
	return s.db.Close()
}

// seal encrypts a file record for storing under the given id.
func (s *BBoltFileStore) seal(id []byte, data []byte) ([]byte, error) {

	if !s.encrypt {
		return data, nil
	}

	return crypto.SealWithSubkey(s.masterKey, id, infoFileRecord, data)
}

// open decrypts a file record stored under the given id.
func (s *BBoltFileStore) open(id []byte, data []byte) ([]byte, error) {

	if !s.encrypt {
		return data, nil
	}

	return crypto.OpenWithSubkey(s.masterKey, id, infoFileRecord, data)
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

		data, err = s.seal(id[:], data)

		if err != nil {
			return err
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

		data, err := s.open(id[:], data)

		if err != nil {
			return err
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

		data, err := s.open(id[:], data)

		if err != nil {
			return err
		}

		if err := file.FromBinary(data); err != nil {
			return err
		}

		if err := mutate(&file); err != nil {
			return err
		}

		data, err = file.ToBinary()

		if err != nil {
			return err
		}

		data, err = s.seal(id[:], data)

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

			v, err := s.open(k, v)

			if err != nil {

				log.Warn().Hex("key", k).Err(err).Msg("unable to decrypt file")

				return nil
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
