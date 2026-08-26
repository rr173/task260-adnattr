package httpapi

import (
	"net/http"
)

// handleCreateLibrary POST /api/libraries
func (s *Server) handleCreateLibrary(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
		Note string `json:"note"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	lb, err := s.svc.CreateLibrary(req.Name, req.Note)
	if err != nil {
		st, msg := statusFromErr(err)
		writeErr(w, st, msg)
		return
	}
	writeJSON(w, http.StatusCreated, lb)
}

// handleListLibraries GET /api/libraries
func (s *Server) handleListLibraries(w http.ResponseWriter, r *http.Request) {
	lbs, err := s.svc.Store.ListLibraries()
	if err != nil {
		st, msg := statusFromErr(err)
		writeErr(w, st, msg)
		return
	}
	writeJSON(w, http.StatusOK, lbs)
}

// handleGetLibrary GET /api/libraries/{id}
func (s *Server) handleGetLibrary(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid library id")
		return
	}
	lb, err := s.svc.Store.GetLibrary(id)
	if err != nil {
		st, msg := statusFromErr(err)
		writeErr(w, st, msg)
		return
	}
	writeJSON(w, http.StatusOK, lb)
}

// handleAdvanceLibrary POST /api/libraries/{id}/advance
func (s *Server) handleAdvanceLibrary(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid library id")
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	lb, err := s.svc.AdvanceLibrary(id, req.Status)
	if err != nil {
		st, msg := statusFromErr(err)
		writeErr(w, st, msg)
		return
	}
	writeJSON(w, http.StatusOK, lb)
}

// handleSealLibrary POST /api/libraries/{id}/seal
func (s *Server) handleSealLibrary(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid library id")
		return
	}
	lb, err := s.svc.SealLibrary(id)
	if err != nil {
		st, msg := statusFromErr(err)
		writeErr(w, st, msg)
		return
	}
	writeJSON(w, http.StatusOK, lb)
}

// handleAnalyzeLibrary POST /api/libraries/{id}/analyze
func (s *Server) handleAnalyzeLibrary(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid library id")
		return
	}
	prof, cand, err := s.svc.Analyze(id)
	if err != nil {
		st, msg := statusFromErr(err)
		writeErr(w, st, msg)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"damage_profile": prof,
		"attribution":    cand,
	})
}

// handleClusterLibrary POST /api/libraries/{id}/cluster
func (s *Server) handleClusterLibrary(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid library id")
		return
	}
	clusters, err := s.svc.Cluster(id)
	if err != nil {
		st, msg := statusFromErr(err)
		writeErr(w, st, msg)
		return
	}
	writeJSON(w, http.StatusCreated, clusters)
}

// handleExcludeBatch POST /api/libraries/{id}/exclude-batch
func (s *Server) handleExcludeBatch(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid library id")
		return
	}
	lb, err := s.svc.ExcludeBatch(id)
	if err != nil {
		st, msg := statusFromErr(err)
		writeErr(w, st, msg)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"library": lb,
		"message": "contamination-suspected clusters excluded",
	})
}
