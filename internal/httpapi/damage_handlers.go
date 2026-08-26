package httpapi

import (
	"net/http"
	"strconv"
)

// handleGetDamageProfile GET /api/damage-profiles?library_id=
func (s *Server) handleGetDamageProfile(w http.ResponseWriter, r *http.Request) {
	libID, err := strconv.ParseInt(r.URL.Query().Get("library_id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "library_id required")
		return
	}
	prof, err := s.svc.Store.GetDamageProfile(libID)
	if err != nil {
		st, msg := statusFromErr(err)
		writeErr(w, st, msg)
		return
	}
	writeJSON(w, http.StatusOK, prof)
}
