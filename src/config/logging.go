package config

import (
	"os"

	"github.com/minnowo/log4zero"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func InitLogging() {

	log4zero.InitOnce("./log-config.json")
	log.Logger = *log4zero.GetNew("", zerolog.InfoLevel, os.Stdout, true)
}
