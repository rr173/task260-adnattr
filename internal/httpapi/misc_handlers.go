package httpapi

import "net/http"

// handleStats GET /api/stats
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.svc.Stats()
	if err != nil {
		st, msg := statusFromErr(err)
		writeErr(w, st, msg)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// handleSelfCheck GET /api/self-check
func (s *Server) handleSelfCheck(w http.ResponseWriter, r *http.Request) {
	result, err := s.svc.SelfCheck()
	if err != nil {
		st, msg := statusFromErr(err)
		writeErr(w, st, msg)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
