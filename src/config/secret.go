package config

import (
	"encoding/hex"
	"os"

	"github.com/rs/zerolog/log"
)

const ENV__SECRET = "KI_MASTER_SECRET"

var envSecret []byte = nil

// Used for encrypting data in the bolt database.
// TODO: Set this at compiletime?
func GetMasterSecret() []byte {

	if envSecret == nil {

		envVal := os.Getenv(ENV__SECRET)

		if envVal == "" || len(envVal) < 2 {
			log.Panic().Str("var", ENV__SECRET).Str("val", envVal).Msg("read empty or invalid secret")
		}

		key, err := hex.DecodeString(envVal)

		if err != nil {
			log.Panic().Err(err).Msg("could not hex decode given secret")
		}

		envSecret = key
	}

	return envSecret
}
