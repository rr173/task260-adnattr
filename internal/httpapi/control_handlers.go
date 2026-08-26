package httpapi

import (
	"net/http"
)

// handleCreateControl POST /api/controls
func (s *Server) handleCreateControl(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string  `json:"name"`
		IsBlank   bool    `json:"is_blank"`
		MeanLen   float64 `json:"mean_len"`
		MeanC2T5p float64 `json:"mean_c2t_5p"`
		MeanG2A3p float64 `json:"mean_g2a_3p"`
		LibraryID int64   `json:"library_id,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	c, err := s.svc.CreateControl(req.Name, req.IsBlank, req.MeanLen, req.MeanC2T5p, req.MeanG2A3p, req.LibraryID)
	if err != nil {
		st, msg := statusFromErr(err)
		writeErr(w, st, msg)
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

// handleListControls GET /api/controls
func (s *Server) handleListControls(w http.ResponseWriter, r *http.Request) {
	cs, err := s.svc.Store.ListControls()
	if err != nil {
		st, msg := statusFromErr(err)
		writeErr(w, st, msg)
		return
	}
	writeJSON(w, http.StatusOK, cs)
}

// handleGetControl GET /api/controls/{id}
func (s *Server) handleGetControl(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid control id")
		return
	}
	c, err := s.svc.Store.GetControl(id)
	if err != nil {
		st, msg := statusFromErr(err)
		writeErr(w, st, msg)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// handleAssociateControl POST /api/controls/{id}/associate
func (s *Server) handleAssociateControl(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid control id")
		return
	}
	var req struct {
		LibraryID int64 `json:"library_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.svc.AssociateControl(req.LibraryID, id); err != nil {
		st, msg := statusFromErr(err)
		writeErr(w, st, msg)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "control associated"})
}
