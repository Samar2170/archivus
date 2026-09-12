package server

import (
	"archivus/internal/config"
	archivus_constants "archivus/internal/constants"
	"archivus/internal/handlers"
	"archivus/internal/services/auth"
	"archivus/internal/services/chunkupload"
	"archivus/pkg/logging"
	"archivus/pkg/response"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

func HealthCheck(w http.ResponseWriter, r *http.Request) {
	response.SuccessResponse(w, "OK")
}

func GetServer(authService *auth.AuthService) *http.Server {
	router := mux.NewRouter()
	router.HandleFunc("/health", HealthCheck)
	authHandler := handlers.NewAuthHandler(authService)

	router.HandleFunc("/auth/login", authHandler.Login).Methods(http.MethodPost)
	router.HandleFunc("/auth/register", authHandler.Register).Methods(http.MethodPost)

	protected := router.NewRoute().Subrouter()
	protected.Use(AuthMiddleware(authService))
	protected.Use(HomeMiddleware(authService))

	protected.HandleFunc("/auth/drive/invite", authHandler.InviteUser).Methods(http.MethodPost)
	protected.HandleFunc("/auth/drive/remove", authHandler.RemoveUserFromDrive).Methods(http.MethodPost)
	protected.HandleFunc("/auth/drive/add", authHandler.AddUserToDrive).Methods(http.MethodPost)

	protected.HandleFunc("/auth/drive/users", authHandler.GetUsersInDrive).Methods(http.MethodGet)
	protected.HandleFunc("/auth/user/info", authHandler.GetUserInfoHandler).Methods(http.MethodGet)
	protected.HandleFunc("/auth/drive/info", authHandler.GetDriveInfoHandler).Methods(http.MethodGet)

	// API keys: created by logged-in users for programmatic clients; the
	// plaintext key is returned only by the create endpoint.
	apiKeyHandler := handlers.NewApiKeyHandler(authService)
	protected.HandleFunc("/auth/apikey/create", apiKeyHandler.CreateApiKey).Methods(http.MethodPost)
	protected.HandleFunc("/auth/apikey/list", apiKeyHandler.GetApiKeys).Methods(http.MethodGet)
	protected.HandleFunc("/auth/apikey/revoke", apiKeyHandler.RevokeApiKey).Methods(http.MethodPost)

	chunkManager := chunkupload.NewManager(config.Config.ArchivusHome)
	storageHandler := handlers.NewStorageHandler(authService.StorageManager, chunkManager)
	protected.HandleFunc("/storage/folder/create", storageHandler.CreateFolder).Methods(http.MethodPost)
	protected.HandleFunc("/storage/folder/delete", storageHandler.DeleteFolder).Methods(http.MethodPost)

	protected.HandleFunc("/storage/file/upload", storageHandler.UploadFileHandler).Methods(http.MethodPost)

	// Resumable chunked upload for large files: init a session, stream chunks
	// (idempotent, so a dropped connection just means re-sending missing chunks),
	// poll status to resume, then complete to assemble and persist.
	protected.HandleFunc("/storage/file/upload/chunk/init", storageHandler.InitChunkUploadHandler).Methods(http.MethodPost)
	protected.HandleFunc("/storage/file/upload/chunk/part", storageHandler.UploadChunkHandler).Methods(http.MethodPost)
	protected.HandleFunc("/storage/file/upload/chunk/status", storageHandler.ChunkUploadStatusHandler).Methods(http.MethodGet)
	protected.HandleFunc("/storage/file/upload/chunk/complete", storageHandler.CompleteChunkUploadHandler).Methods(http.MethodPost)
	protected.HandleFunc("/storage/file/upload/chunk/abort", storageHandler.AbortChunkUploadHandler).Methods(http.MethodPost)
	protected.HandleFunc("/storage/file/download", storageHandler.DownloadFileHandler).Methods(http.MethodGet)
	// Mints a direct object-storage download URL; returns "" on backends
	// without one (local disk), where the client streams via the route above.
	protected.HandleFunc("/storage/file/download/url", storageHandler.DownloadURLHandler).Methods(http.MethodGet)
	protected.HandleFunc("/storage/file/move", storageHandler.MoveFileHandler).Methods(http.MethodPost)
	protected.HandleFunc("/storage/file/delete", storageHandler.DeleteFileHandler).Methods(http.MethodPost)
	protected.HandleFunc("/storage/files", storageHandler.GetFilesHandler).Methods(http.MethodPost)

	protected.HandleFunc("/storage/recyclebin", storageHandler.GetRecycleBinHandler).Methods(http.MethodPost)
	protected.HandleFunc("/storage/recyclebin/restore", storageHandler.RestoreFileHandler).Methods(http.MethodPost)
	// Permanently deletes a recycle bin item (file or folder) on demand,
	// bypassing the remaining retention window.
	protected.HandleFunc("/storage/recyclebin/purge", storageHandler.PurgeRecycleBinItemHandler).Methods(http.MethodPost)

	// Shared folders: a dedicated read-only surface for folders other users
	// shared with the current user. Drive-membership storage APIs above are
	// untouched by it.
	sharedHandler := handlers.NewSharedHandler(authService.Store, authService.StorageManager)
	protected.HandleFunc("/storage/shared/roots", sharedHandler.GetRoots).Methods(http.MethodGet)
	protected.HandleFunc("/storage/shared/list", sharedHandler.ListFiles).Methods(http.MethodPost)
	protected.HandleFunc("/storage/shared/file/download", sharedHandler.DownloadFile).Methods(http.MethodGet)
	protected.HandleFunc("/storage/shared/file/download/url", sharedHandler.DownloadURL).Methods(http.MethodGet)
	protected.HandleFunc("/storage/shared/grant", sharedHandler.GrantShare).Methods(http.MethodPost)
	protected.HandleFunc("/storage/shared/revoke", sharedHandler.RevokeShare).Methods(http.MethodPost)
	protected.HandleFunc("/storage/shared/list-users", sharedHandler.ListSharedUsers).Methods(http.MethodPost)

	// Serve generated thumbnail images as static files. Thumbnails are loaded via
	// <img> tags which cannot send Authorization headers, so this route is public;
	// it only ever exposes downscaled preview JPEGs from the thumbnail directory.
	thumbFS := http.FileServer(http.Dir(config.Config.ThumbnailDir))
	router.PathPrefix(archivus_constants.ThumbnailRoutePrefix).Handler(
		http.StripPrefix(archivus_constants.ThumbnailRoutePrefix, thumbFS),
	).Methods(http.MethodGet)

	// Anything that is not an API route is the frontend's to route. Serving it
	// from the same origin means the browser never needs CORS.
	if staticDir := DefaultStaticDir(); hasFrontend(staticDir) {
		log.Printf("serving frontend from %s", staticDir)
		router.NotFoundHandler = SPAHandler(staticDir)
	} else {
		log.Printf("no frontend found at %s, running API only", staticDir)
	}

	return &http.Server{Handler: logging.NewLogMiddleware().Handler(CORSMiddleware(router)), Addr: ":8080"}
}
