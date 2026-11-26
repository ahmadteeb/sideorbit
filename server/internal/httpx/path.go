package httpx

import (
	"net/http"

	"github.com/google/uuid"
)

// NotFound answers 404. Records owned by someone else get the same answer as
// records that don't exist.
func NotFound(w http.ResponseWriter) {
	Error(w, http.StatusNotFound, "not_found", "That doesn't exist, or isn't yours.", "")
}

// ID parses a UUID path value, answering 404 if it is malformed.
func ID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue(name))
	if err != nil {
		NotFound(w)
		return uuid.Nil, false
	}
	return id, true
}
