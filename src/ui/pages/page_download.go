package pages

import (
	"fmt"
	"ki/src/handlers/storage"
	"time"
)

type PageDownloadFileView struct {
	BaseView
	File   *storage.KiMetadata
	FileID storage.FileID
}

func formatDuration(d time.Duration) string {
	totalSeconds := int64(d.Seconds())

	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
}
