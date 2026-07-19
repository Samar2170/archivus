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
	// ThumbnailRoutePrefix is the URL path prefix under which thumbnail images
	// are served by the static file server. Must end with a trailing slash.
	ThumbnailRoutePrefix = "/storage/thumbnails/"

	MaxUploadSize = 100 << 20 // 100 MB
)

type ContextKey string
