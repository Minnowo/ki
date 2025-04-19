package crypto

import (
	"crypto/sha256"
)

func OneTimePad(serverKey, userKey []byte) ([]byte, error) {

	kSize := AESKeySize(len(serverKey))
	kSize.Assert()

	if len(userKey) != int(kSize) {

		// make sure the users input is always the right size
		sha := sha256.Sum256(userKey)
		userKey = sha[0:kSize]
	}

	var key []byte = make([]byte, kSize)

	for i := range kSize {

		key[i] = userKey[i] ^ serverKey[i]
	}

	return key, nil
}
