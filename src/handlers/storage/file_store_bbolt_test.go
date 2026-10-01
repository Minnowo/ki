package storage

import (
	"ki/src/config"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	bolt "go.etcd.io/bbolt"
)

func init() {
	// deriving the master key at full strength for every test store is too slow
	config.SetMasterKeyRounds(1000)
}

func testBoltFile() *KiFile {
	return &KiFile{
		KiMetadata: KiMetadata{
			Size:             1024,
			Expires:          time.Now().Add(time.Hour),
			AllowedDownloads: 10,
			Name:             "file.txt",
		},
		FilePath: "/tmp/file",
		Key:      make([]byte, 32),
		KeySize:  32,
	}
}

func TestBBoltFileStore_Reopen(t *testing.T) {

	dbPath := path.Join(t.TempDir(), "data.db")

	store, err := newBBoltFileStore(dbPath, "correct secret")
	assert.NoError(t, err)

	id, err := store.StoreFile(testBoltFile())
	assert.NoError(t, err)
	assert.NoError(t, store.Close())

	t.Run("same secret reads existing files", func(t *testing.T) {
		store, err := newBBoltFileStore(dbPath, "correct secret")
		assert.NoError(t, err)
		defer store.Close()

		file, ok := store.GetFile(id)
		assert.True(t, ok)
		assert.Equal(t, "file.txt", file.Name)
	})

	t.Run("wrong secret is refused", func(t *testing.T) {
		store, err := newBBoltFileStore(dbPath, "wrong secret")
		assert.ErrorIs(t, err, ErrWrongMasterSecret)
		assert.Nil(t, store)
	})

	t.Run("keeps the rounds it was created with", func(t *testing.T) {
		old := config.MasterKeyRounds()
		config.SetMasterKeyRounds(2000)
		defer config.SetMasterKeyRounds(old)

		store, err := newBBoltFileStore(dbPath, "correct secret")
		assert.NoError(t, err)
		defer store.Close()

		_, ok := store.GetFile(id)
		assert.True(t, ok, "changing the default rounds should not break existing databases")
	})
}

func TestBBoltFileStore_NewDatabasesGetDifferentSalts(t *testing.T) {

	dir := t.TempDir()

	a, err := newBBoltFileStore(path.Join(dir, "a.db"), "secret")
	assert.NoError(t, err)
	defer a.Close()

	b, err := newBBoltFileStore(path.Join(dir, "b.db"), "secret")
	assert.NoError(t, err)
	defer b.Close()

	assert.NotEqual(t, a.masterKey, b.masterKey, "the same secret should give different master keys for different databases")
}

func TestBBoltFileStore_RecordsAreBoundToTheirID(t *testing.T) {

	store, err := newBBoltFileStore(path.Join(t.TempDir(), "data.db"), "secret")
	assert.NoError(t, err)
	defer store.Close()

	a, err := store.StoreFile(testBoltFile())
	assert.NoError(t, err)

	b, err := store.StoreFile(testBoltFile())
	assert.NoError(t, err)

	// copy a's record over b's, each record has its own subkey so it should not decrypt as b
	err = store.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bBucketFiles)
		return bucket.Put(b[:], bucket.Get(a[:]))
	})
	assert.NoError(t, err)

	_, ok := store.GetFile(b)
	assert.False(t, ok, "a record moved to another id should not decrypt")

	_, ok = store.GetFile(a)
	assert.True(t, ok)
}

func TestBBoltFileStore_LegacyDatabase(t *testing.T) {

	dbPath := path.Join(t.TempDir(), "data.db")

	// a database from before the meta bucket, with a file in it
	db, err := bolt.Open(dbPath, 0600, nil)
	assert.NoError(t, err)

	err = db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucket(bBucketFiles)
		if err != nil {
			return err
		}
		return b.Put([]byte("0123456789abcdef"), []byte("old encrypted record"))
	})
	assert.NoError(t, err)
	assert.NoError(t, db.Close())

	store, err := newBBoltFileStore(dbPath, "secret")
	assert.ErrorIs(t, err, ErrLegacyDatabase)
	assert.Nil(t, store)

	// the refused open must not have changed the database
	db, err = bolt.Open(dbPath, 0600, nil)
	assert.NoError(t, err)
	defer db.Close()

	err = db.View(func(tx *bolt.Tx) error {
		assert.Nil(t, tx.Bucket(bBucketMeta), "meta bucket should not be created")
		assert.NotNil(t, tx.Bucket(bBucketFiles).Get([]byte("0123456789abcdef")), "old record should be untouched")
		return nil
	})
	assert.NoError(t, err)
}

func TestBBoltFileStore_EmptyLegacyDatabase(t *testing.T) {

	dbPath := path.Join(t.TempDir(), "data.db")

	// a database from before the meta bucket, with no files in it
	db, err := bolt.Open(dbPath, 0600, nil)
	assert.NoError(t, err)

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket(bBucketFiles)
		return err
	})
	assert.NoError(t, err)
	assert.NoError(t, db.Close())

	store, err := newBBoltFileStore(dbPath, "secret")
	assert.NoError(t, err, "an empty old database has nothing to lose, so it should be upgraded")
	defer store.Close()
}
