package httpapi

import (
	"net/http"
	"strconv"
)

// handleListAttributions GET /api/attributions?library_id=
func (s *Server) handleListAttributions(w http.ResponseWriter, r *http.Request) {
	libID, err := strconv.ParseInt(r.URL.Query().Get("library_id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "library_id required")
		return
	}
	attrs, err := s.svc.Store.ListAttributionsByLibrary(libID)
	if err != nil {
		st, msg := statusFromErr(err)
		writeErr(w, st, msg)
		return
	}
	writeJSON(w, http.StatusOK, attrs)
}

// handleGetAttribution GET /api/attributions/{id}
func (s *Server) handleGetAttribution(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid attribution id")
		return
	}
	a, err := s.svc.Store.GetAttribution(id)
	if err != nil {
		st, msg := statusFromErr(err)
		writeErr(w, st, msg)
		return
	}
	writeJSON(w, http.StatusOK, a)
}

// handleConfirmAttribution POST /api/attributions/{id}/confirm
func (s *Server) handleConfirmAttribution(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid attribution id")
		return
	}
	a, err := s.svc.ConfirmAttribution(id)
	if err != nil {
		st, msg := statusFromErr(err)
		writeErr(w, st, msg)
		return
	}
	writeJSON(w, http.StatusOK, a)
}
