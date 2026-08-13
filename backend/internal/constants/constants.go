package archivus_constants

import "time"

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

	// MaxChunkSize caps the size of a single chunk in a chunked upload. Clients
	// should keep individual chunks well under this; the whole assembled file is
	// still bounded by MaxUploadSize at completion time.
	MaxChunkSize = 64 << 20 // 64 MB

	// ChunkStagingDirName is the directory under the archivus home where
	// in-progress chunked uploads are staged before assembly.
	ChunkStagingDirName = ".chunks"

	// ChunkSessionTTL is how long an unfinished chunked upload session is kept
	// before the staging GC reclaims it.
	//
	// It is deliberately longer than the sync client's own resume window: a
	// client that comes back aborts its dead sessions itself, which is the clean
	// path, and the GC only has to cover clients that never return at all (lost
	// state file, reinstalled machine, abandoned upload).
	ChunkSessionTTL = 8 * 24 * time.Hour

	// AssembledUploadTTL is the grace period before an assembled file that no
	// file row points at is treated as garbage. Assembling the file and creating
	// the row that claims it are not atomic, so this only has to be long enough
	// to cover that gap; the reference check does the real work.
	AssembledUploadTTL = 24 * time.Hour

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

	// MinStageHeadroomBytes is the free space the chunk staging area insists on
	// keeping available before it will accept a new session, write another chunk,
	// or assemble a file. Assembly puts a second full copy of the file on disk on
	// top of the session's already-staged chunks, so an upload can transiently
	// need ~2x its size. Checking free space up front turns what would otherwise
	// be a silent ENOSPC mid-write into a clean, catchable refusal before any
	// bytes land.
	MinStageHeadroomBytes = 512 << 20 // 512 MB
)

type ContextKey string
