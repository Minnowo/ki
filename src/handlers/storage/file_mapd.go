package storage

import (
	"time"

	"github.com/rs/zerolog/log"
)

func fileMapExpireCheckD(interval time.Duration, done chan bool, fm *FileMap) {

	log.Debug().Msg("fileMapExpireCheckD starting")

	ticker := time.NewTicker(interval)

	for {

		select {
		case <-done:
			log.Debug().Msg("fileMapExpireCheckD done")
			return

		case t := <-ticker.C:

			fm.RemoveExpired()
			log.Debug().Str("time", t.String()).Msg("clearing out expired files")
		}

	}
}

func NewFileMapExpireCheckD(interval time.Duration, fm *FileMap) chan bool {

	done := make(chan bool)

	go fileMapExpireCheckD(interval, done, fm)

	return done
}
