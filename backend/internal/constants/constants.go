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

	// RecycleBinDirName is the directory (disk) / key prefix (S3) under the
	// storage root where deleted files are held before permanent removal.
	RecycleBinDirName = ".recyclebin"
	// RecycleBinRetentionDays is how long a deleted file stays in the recycle
	// bin before the purge job removes it permanently.
	RecycleBinRetentionDays = 30
	// ThumbnailRoutePrefix is the URL path prefix under which thumbnail images
	// are served by the static file server. Must end with a trailing slash.
	ThumbnailRoutePrefix = "/storage/thumbnails/"

	MaxUploadSize = 2 << 30 // 2 GB

	// MaxMultipartMemory caps how much of a multipart upload is buffered in
	// memory during parsing; the remainder is spooled to temp files on disk.
	MaxMultipartMemory = 32 << 20 // 32 MB
)

type ContextKey string
