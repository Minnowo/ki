package crypto

import (
	"crypto/rand"
	mrand "math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

func newTestMasterKey(t *testing.T, kSize AESKeySize) []byte {
	t.Helper()

	salt := make([]byte, 32)
	rand.Read(salt)

	key, err := DeriveMasterKey(kSize, "this is a master secret", salt, 1000)
	assert.Nil(t, err, "should derive master key")
	assert.Len(t, key, int(kSize))

	return key
}

func TestDeriveMasterKey(t *testing.T) {

	salt := []byte("salt")

	a, err := DeriveMasterKey(AES256, "secret", salt, 1000)
	assert.Nil(t, err)

	b, err := DeriveMasterKey(AES256, "secret", salt, 1000)
	assert.Nil(t, err)
	assert.Equal(t, a, b, "same inputs should derive the same key")

	c, err := DeriveMasterKey(AES256, "secret", []byte("other salt"), 1000)
	assert.Nil(t, err)
	assert.NotEqual(t, a, c, "different salts should derive different keys")

	_, err = DeriveMasterKey(AESKeySize(31), "secret", salt, 1000)
	assert.ErrorIs(t, err, ErrInvalidKeySize)
}

func TestSealWithSubkey(t *testing.T) {

	plainText := []byte("This is a test message")

	tests := []struct {
		name    string
		keySize AESKeySize
	}{
		{"AES128", AES128},
		{"AES192", AES192},
		{"AES256", AES256},
	}

	for _, tt := range tests {

		masterKey := newTestMasterKey(t, tt.keySize)

		t.Run(tt.name+" all ok", func(t *testing.T) {

			data, err := SealWithSubkey(masterKey, []byte("salt"), "info", plainText)
			assert.Nil(t, err, "should encrypt data")

			decrypted, err := OpenWithSubkey(masterKey, []byte("salt"), "info", data)
			assert.Nil(t, err, "should decrypt data")
			assert.Equal(t, plainText, decrypted, "should match")
		})

		t.Run(tt.name+" wrong salt, info, or key", func(t *testing.T) {

			data, err := SealWithSubkey(masterKey, []byte("salt"), "info", plainText)
			assert.Nil(t, err, "should encrypt data")

			_, err = OpenWithSubkey(masterKey, []byte("other salt"), "info", data)
			assert.NotNil(t, err, "should not decrypt with a different salt")

			_, err = OpenWithSubkey(masterKey, []byte("salt"), "other info", data)
			assert.NotNil(t, err, "should not decrypt with different info")

			_, err = OpenWithSubkey(newTestMasterKey(t, tt.keySize), []byte("salt"), "info", data)
			assert.NotNil(t, err, "should not decrypt with a different master key")
		})

		t.Run(tt.name+" with tampered data", func(t *testing.T) {

			for range 100 {
				data, err := SealWithSubkey(masterKey, []byte("salt"), "info", plainText)
				assert.Nil(t, err, "should encrypt data")

				// Flip a random bit in the ciphertext
				idx := mrand.Intn(len(data))
				bit := byte(1 << uint(mrand.Intn(8)))
				data[idx] ^= bit

				_, err = OpenWithSubkey(masterKey, []byte("salt"), "info", data)
				assert.NotNil(t, err, "should get error for tampered data")
			}
		})
	}

	t.Run("invalid master key size", func(t *testing.T) {
		_, err := SealWithSubkey(make([]byte, 31), []byte("salt"), "info", plainText)
		assert.ErrorIs(t, err, ErrInvalidKeySize)
	})
}
