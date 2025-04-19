package logging

import (
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	zerologger "github.com/rs/zerolog/log"
)

var (
	ApiLog zerolog.Logger = zerologger.Logger
)

func initLogger(name string, defaultLevel zerolog.Level) zerolog.Logger {

	levelStr := strings.TrimSpace(os.Getenv(name + "_LEVEL"))
	color := strings.TrimSpace(os.Getenv(name+"_NO_COLOR")) != "true"

	var level zerolog.Level
	var err error

	if len(levelStr) == 0 {
		level = defaultLevel
	} else {

		level, err = zerolog.ParseLevel(levelStr)

		if err != nil {
			level = defaultLevel
		}
	}

	cw := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339, NoColor: !color}

	logger := zerologger.Output(cw).Level(level)
	logger.WithLevel(zerolog.DebugLevel).Str("name", name).Msg("logger ready")
	return logger
}

func init() {

	// global logger
	zerologger.Logger = initLogger("LOG", zerolog.InfoLevel)

	// all api response calls
	ApiLog = initLogger("API_LOG", zerolog.DebugLevel)
}

func Init() {}
