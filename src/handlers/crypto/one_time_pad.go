package crypto

import (
	"crypto/sha256"
	"fmt"

	"github.com/rs/zerolog/log"
)

func OneTimePad(kSize AESKeySize, serverKey, userKey []byte) ([]byte, error) {

	if len(serverKey) != int(kSize) {
		log.Error().Int("keySize", int(kSize)).Int("serverKeyLen", len(serverKey)).Msg("Trying one time pad with invalid key size or key length")
		return nil, fmt.Errorf("Key size does not match the server key")
	}

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
