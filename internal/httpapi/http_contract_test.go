package httpapi_test

import (
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
