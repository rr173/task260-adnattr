package httpapi

import (
	"net/http"
	"strconv"

	"task260-adnattr/internal/service"
)

// handleIngestFragment POST /api/fragments
func (s *Server) handleIngestFragment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LibraryID     int64   `json:"library_id"`
		FragLen       int     `json:"frag_len"`
		C2T5p         float64 `json:"c2t_5p"`
		G2A3p         float64 `json:"g2a_3p"`
		MeanBaseError float64 `json:"mean_base_error"`
		Sequence      string  `json:"sequence,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	fs, added, err := s.svc.IngestFragment(req.LibraryID, req.FragLen, req.C2T5p, req.G2A3p, req.MeanBaseError, req.Sequence)
	if err != nil {
		st, msg := statusFromErr(err)
		writeErr(w, st, msg)
		return
	}
	status := http.StatusCreated
	if !added {
		status = http.StatusOK
	}
	writeJSON(w, status, map[string]interface{}{
		"fragment": fs,
		"added":    added,
	})
}

// handleBatchIngestFragments POST /api/fragments/batch
func (s *Server) handleBatchIngestFragments(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LibraryID int64 `json:"library_id"`
		Items     []struct {
			FragLen       int     `json:"frag_len"`
			C2T5p         float64 `json:"c2t_5p"`
			G2A3p         float64 `json:"g2a_3p"`
			MeanBaseError float64 `json:"mean_base_error"`
			Sequence      string  `json:"sequence,omitempty"`
		} `json:"items"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	inputs := make([]service.FragmentInput, 0, len(req.Items))
	for _, it := range req.Items {
		inputs = append(inputs, service.FragmentInput{FragLen: it.FragLen, C2T5p: it.C2T5p, G2A3p: it.G2A3p, MeanBaseError: it.MeanBaseError, Sequence: it.Sequence})
	}
	added, ignored, err := s.svc.IngestFragmentsAtomic(req.LibraryID, inputs)
	if err != nil {
		st, msg := statusFromErr(err)
		writeErr(w, st, msg)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"library_id": req.LibraryID,
		"added":      added,
		"ignored":    ignored,
	})
}

// handleListFragments GET /api/fragments?library_id=
func (s *Server) handleListFragments(w http.ResponseWriter, r *http.Request) {
	libID, err := strconv.ParseInt(r.URL.Query().Get("library_id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "library_id required")
		return
	}
	frags, err := s.svc.Store.ListFragmentsByLibrary(libID)
	if err != nil {
		st, msg := statusFromErr(err)
		writeErr(w, st, msg)
		return
	}
	writeJSON(w, http.StatusOK, frags)
}

// handleGetFragment GET /api/fragments/{id}
func (s *Server) handleGetFragment(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid fragment id")
		return
	}
	fs, err := s.svc.Store.GetFragment(id)
	if err != nil {
		st, msg := statusFromErr(err)
		writeErr(w, st, msg)
		return
	}
	writeJSON(w, http.StatusOK, fs)
}

// handleListClusters GET /api/fragment-clusters?library_id=
func (s *Server) handleListClusters(w http.ResponseWriter, r *http.Request) {
	libID, err := strconv.ParseInt(r.URL.Query().Get("library_id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "library_id required")
		return
	}
	clusters, err := s.svc.Store.ListClustersByLibrary(libID)
	if err != nil {
		st, msg := statusFromErr(err)
		writeErr(w, st, msg)
		return
	}
	writeJSON(w, http.StatusOK, clusters)
}

// handleGetCluster GET /api/fragment-clusters/{id}
func (s *Server) handleGetCluster(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid cluster id")
		return
	}
	c, err := s.svc.Store.GetCluster(id)
	if err != nil {
		st, msg := statusFromErr(err)
		writeErr(w, st, msg)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// handleClassifyCluster POST /api/fragment-clusters/{id}/classify
func (s *Server) handleClassifyCluster(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid cluster id")
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	c, err := s.svc.ClassifyCluster(id, req.Status)
	if err != nil {
		st, msg := statusFromErr(err)
		writeErr(w, st, msg)
		return
	}
	writeJSON(w, http.StatusOK, c)
}
