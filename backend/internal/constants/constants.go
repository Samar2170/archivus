package archivus_constants

const (
	UserId            = "userId"
	UserIdKey         = "userId"
	RequestIdKey      = "requestId"
	RequestIdHeader   = "X-Request-Id"
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

	// MaxChunkSize caps the size of a single chunk in a chunked upload. Clients
	// should keep individual chunks well under this; the whole assembled file is
	// still bounded by MaxUploadSize at completion time.
	MaxChunkSize = 64 << 20 // 64 MB

	// ChunkStagingDirName is the directory under the archivus home where
	// in-progress chunked uploads are staged before assembly.
	ChunkStagingDirName = ".chunks"

	// PendingUploadWorkers is how many staged uploads one drain pushes to object
	// storage at a time. Each push is itself a concurrent multipart upload, so
	// the number of in-flight streams is this times that concurrency; raising it
	// past what the uplink can carry only spreads the same bandwidth thinner.
	PendingUploadWorkers = 4

	// PendingUploadBatchSize is how many pending rows a drain claims per pass. A
	// drain keeps taking passes until nothing new turns up, so this bounds how
	// many rows are held in memory at once, not how many a drain gets through.
	PendingUploadBatchSize = 20

	// MaxPendingUploadBacklogMB bounds the assembled-but-not-yet-pushed bytes
	// allowed to sit in local staging. Once the backlog is this large, completing
	// another chunked upload is refused so clients slow down instead of filling
	// the disk faster than the workers can drain it.
	MaxPendingUploadBacklogMB = 20 * 1024 // 20 GB
)

type ContextKey string

var FilteringExtensionMap = map[string][]string{
	"images": {
		"jpg",
		"jpeg",
		"png",
		"gif",
		"webp",
		"bmp",
		"tiff",
		"svg",
		"heic",
	},
	"videos": {
		"mp4",
		"mov",
		"mkv",
		"avi",
		"webm",
		"m4v",
	},
	"audio": {
		"mp3",
		"wav",
		"flac",
		"ogg",
		"m4a",
	},
	"spreadsheets": {
		"xls",
		"xlsx",
		"csv",
		"ods",
		"odt",
	},
	"docs": {
		"pdf",
		"doc",
		"docx",
		"txt",
	},
	"code": {
		"py",
		"js",
		"sh",
		"go",
		"java",
		"c",
		"cpp",
		"rs",
		"ts",
		"rb",
		"php",
		"html",
		"css",
		"json",
		"xml",
		"yml",
		"yaml",
	},
}

func GetAllExtensions() []string {
	var allExts []string
	for _, exts := range FilteringExtensionMap {
		allExts = append(allExts, exts...)
	}
	return allExts
}
