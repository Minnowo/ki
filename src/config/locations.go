package config

import (
	"os"

	"github.com/rs/zerolog/log"
)

var (
	fileStorageDir = "./ki_files"
)

func FileStorageDir() string {
	return fileStorageDir
}
func SetFileStorageDir(dir string) {

	if err := os.MkdirAll(dir, 0700); err != nil {
		log.Panic().Err(err).Msg("could not create storage directory")
	}
	fileStorageDir = dir

}
