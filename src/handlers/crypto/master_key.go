package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/pbkdf2"
	"crypto/sha256"
)

// MASTER_KEY_ROUNDS is the number of PBKDF2 rounds used to turn the master secret into the master key.
// This is only done once on startup, so it can be slow.
const MASTER_KEY_ROUNDS int = 600_000

// DeriveMasterKey stretches the master secret into a key of the given size.
func DeriveMasterKey(kSize AESKeySize, secret string, salt []byte, rounds int) ([]byte, error) {

	if !kSize.Assert() {
		return nil, ErrInvalidKeySize
	}

	return pbkdf2.Key(sha256.New, secret, salt, rounds, int(kSize))
}

// subkeyCipher derives a subkey from the master key for the given salt and info, and returns an AES-GCM cipher for it.
//
// Every salt gets its own subkey, so the limit of 2^32 random nonces applies to each salt rather than to the master key.
// Info separates subkeys used for different purposes.
func subkeyCipher(masterKey []byte, salt []byte, info string) (cipher.AEAD, error) {

	if !AESKeySize(len(masterKey)).Assert() {
		return nil, ErrInvalidKeySize
	}

	subkey, err := hkdf.Key(sha256.New, masterKey, salt, info, len(masterKey))

	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(subkey)

	if err != nil {
		return nil, err
	}

	return cipher.NewGCMWithRandomNonce(block)
}

// SealWithSubkey encrypts and authenticates data with a subkey of the master key. See subkeyCipher.
func SealWithSubkey(masterKey []byte, salt []byte, info string, data []byte) ([]byte, error) {

	aesgcm, err := subkeyCipher(masterKey, salt, info)

	if err != nil {
		return nil, err
	}

	return aesgcm.Seal(nil, nil, data, nil), nil
}

// OpenWithSubkey decrypts data from SealWithSubkey. The salt and info must match the ones used to seal it.
func OpenWithSubkey(masterKey []byte, salt []byte, info string, data []byte) ([]byte, error) {

	aesgcm, err := subkeyCipher(masterKey, salt, info)

	if err != nil {
		return nil, err
	}

	return aesgcm.Open(nil, nil, data, nil)
}
