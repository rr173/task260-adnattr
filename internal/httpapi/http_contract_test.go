package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"task260-adnattr/internal/httpapi"
	"task260-adnattr/internal/service"
	"task260-adnattr/internal/store"
)

func TestHTTPHealthEndpointsReturnJSON(t *testing.T) {
	st, err := store.OpenStore(t.TempDir() + "/http.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	svc := service.New(st)

	h := httpapi.New(svc).Handler()
	for _, path := range []string{"/api/stats", "/api/self-check"} {
		req := httptest.NewRequest("GET", path, nil)
		resp := httptest.NewRecorder()
		h.ServeHTTP(resp, req)
		if resp.Code != 200 {
			t.Fatalf("GET %s status = %d, want 200", path, resp.Code)
		}
		if got := resp.Header().Get("Content-Type"); got == "" {
			t.Fatalf("GET %s missing content type", path)
		}
	}
}

// TestBatchIngestLeavesNoPartialRowsOnInvalidBase 回归测试：批量导入时只要有一条
// 数据含非法碱基，整个批次都不应写入任何片段，便于调用方安全重试。
func TestBatchIngestLeavesNoPartialRowsOnInvalidBase(t *testing.T) {
	st, err := store.OpenStore(t.TempDir() + "/batch.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	svc := service.New(st)
	h := httpapi.New(svc).Handler()

	// 先建库。
	libResp := httptest.NewRecorder()
	libReq := httptest.NewRequest("POST", "/api/libraries",
		bytes.NewReader(mustJSON(t, map[string]string{"name": "L"})))
	h.ServeHTTP(libResp, libReq)
	if libResp.Code != http.StatusCreated {
		t.Fatalf("create library: status %d, body %s", libResp.Code, libResp.Body.String())
	}
	var lib struct{ ID int64 `json:"id"` }
	if err := json.Unmarshal(libResp.Body.Bytes(), &lib); err != nil {
		t.Fatalf("decode library: %v", err)
	}

	// 前两条合法，第三条含非法碱基 N。
	body := mustJSON(t, map[string]interface{}{
		"library_id": lib.ID,
		"items": []map[string]interface{}{
			{"frag_len": 40, "c2t_5p": 0.10, "g2a_3p": 0.05, "mean_base_error": 0.01, "sequence": "ACGTACGT"},
			{"frag_len": 60, "c2t_5p": 0.20, "g2a_3p": 0.15, "mean_base_error": 0.01, "sequence": "ACGTACGTAC"},
			{"frag_len": 0, "c2t_5p": 0.0, "g2a_3p": 0.0, "mean_base_error": 0.01, "sequence": "ACGTN"},
		},
	})
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, httptest.NewRequest("POST", "/api/fragments/batch", bytes.NewReader(body)))
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("batch status = %d, want %d, body %s", resp.Code, http.StatusBadRequest, resp.Body.String())
	}

	// 关键断言：库里不应残留任何片段。
	var count int
	if err := st.DB().QueryRow("SELECT COUNT(*) FROM fragment_summaries WHERE library_id = ?", lib.ID).Scan(&count); err != nil {
		t.Fatalf("count fragments: %v", err)
	}
	if count != 0 {
		t.Fatalf("fragment count = %d, want 0 (batch must be atomic, no partial writes left for safe retry)", count)
	}
}

// mustJSON marshals v or fails the test.
func mustJSON(t *testing.T, v interface{}) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}
