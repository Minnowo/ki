package config

import "github.com/rs/zerolog/log"

// This is the password used for encryption of data inside the bolt database.
// It is a string because the pbkdf2 function wants a string for the password.
// TODO: combine this value with a compile time constant?? Would be harder to get.
var envSecret string

func SetMasterSecret(s string) {

	if s == "" || len(s) < 8 {
		log.Panic().Msg("secret must be at least 8 characters")
	}
	envSecret = s
}

func GetMasterSecret() string {
	return envSecret
}
