package pages

import (
	"ki/src/handlers/storage"
)

type PageDownloadFileView struct {
	BaseView
	File   *storage.KiMetadata
	FileID storage.FileID
}
