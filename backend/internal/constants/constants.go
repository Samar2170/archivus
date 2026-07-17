package archivus_constants

const (
	UserId            = "userId"
	UserIdKey         = "userId"
	StorageDbFile     = "storage.db"
	MinPasswordLength = 8
	PINLength         = 6

	SettingsDir    = ".archivus"
	ConfigFileName = "config.yaml"

	ThumbnailDirName = ".thumbnails"

	MaxUploadSize = 100 << 20 // 100 MB
)

type ContextKey string
