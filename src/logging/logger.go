package logging

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Init initializes the logger
func Init(lvl zerolog.Level, color bool) {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339, NoColor: !color}).Level(lvl)
	log.Debug().Msg("Logger initialized")
}

func InitFromEnv() {

	var zlevel LogLevel

	level := os.Getenv("LOG_LEVEL")
	color := os.Getenv("LOG_NO_COLOR") != "true"

	err := zlevel.Decode(level)

	if err == nil && len(level) > 0 {

		Init(zlevel.AsZeroLogLevel(), color)

	} else {

		Init(zerolog.InfoLevel, color)
	}
}
