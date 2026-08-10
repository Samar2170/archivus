package response

import (
	"encoding/json"
	"net/http"
	"strconv"
)

func JSONResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
}

func DataResponse() {}

func SuccessResponse(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": message,
	})
}

func BadRequestResponse(w http.ResponseWriter, err string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Bad Request",
		"error":   err,
	})
}
func UnauthorizedResponse(w http.ResponseWriter, err string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Unauthorized",
		"error":   err,
	})
}

func NotFoundResponse(w http.ResponseWriter, err string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Not Found",
		"error":   err,
	})
}

func ForbiddenResponse(w http.ResponseWriter, err string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Forbidden",
		"error":   err,
	})
}

func InternalServerErrorResponse(w http.ResponseWriter, err string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Internal Server Error",
		"error":   err,
	})
}

func MethodNotAllowedResponse(w http.ResponseWriter, err string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusMethodNotAllowed)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Method Not Allowed",
		"error":   err,
	})
}

// ServiceUnavailableResponse reports that the server is temporarily unable to
// take the request and that retrying later is expected to work. retryAfter is
// advertised to the client in the Retry-After header; pass 0 to omit it.
func ServiceUnavailableResponse(w http.ResponseWriter, err string, retryAfter int) {
	w.Header().Set("Content-Type", "application/json")
	if retryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
	}
	w.WriteHeader(http.StatusServiceUnavailable)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Service Unavailable",
		"error":   err,
	})
}

// BadRequestResponse 400 ( params, token wrong, )
// UnauthorizedResponse 401 ( token expired, token wrong, )
// ForbiddenResponse 403 ( token expired, token wrong, )
// NotFoundResponse 404 ( user not found, device not found, )
// InternalServerErrorResponse 500 ( db error, )

// SuccessResponse 200 ( user created, device created, device authorized, )
// DataResponse 200 ( user data, device data, )

// ServiceUnavailableResponse 503 ( db error, )
// NoContentResponse 204 ( user deleted, device deleted, )
