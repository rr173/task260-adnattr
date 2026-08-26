package httpapi

import (
	"net/http"
	"strconv"
)

// handlePublishSnapshot POST /api/snapshots
// 请求体：{"library_id": 1, "control_id": 2} —— 发布时冻结对照批次。
func (s *Server) handlePublishSnapshot(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LibraryID int64 `json:"library_id"`
		ControlID int64 `json:"control_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	snap, err := s.svc.PublishSnapshot(req.LibraryID, req.ControlID)
	if err != nil {
		st, msg := statusFromErr(err)
		writeErr(w, st, msg)
		return
	}
	writeJSON(w, http.StatusCreated, snap)
}

// handleListSnapshots GET /api/snapshots?library_id=
func (s *Server) handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	libID, err := strconv.ParseInt(r.URL.Query().Get("library_id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "library_id required")
		return
	}
	snaps, err := s.svc.Store.ListSnapshotsByLibrary(libID)
	if err != nil {
		st, msg := statusFromErr(err)
		writeErr(w, st, msg)
		return
	}
	writeJSON(w, http.StatusOK, snaps)
}

// handleGetSnapshot GET /api/snapshots/{id}
func (s *Server) handleGetSnapshot(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid snapshot id")
		return
	}
	snap, err := s.svc.Store.GetSnapshot(id)
	if err != nil {
		st, msg := statusFromErr(err)
		writeErr(w, st, msg)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

// handleSupersedeSnapshot POST /api/snapshots/{id}/supersede
func (s *Server) handleSupersedeSnapshot(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid snapshot id")
		return
	}
	snap, err := s.svc.SupersedeSnapshot(id)
	if err != nil {
		st, msg := statusFromErr(err)
		writeErr(w, st, msg)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}
