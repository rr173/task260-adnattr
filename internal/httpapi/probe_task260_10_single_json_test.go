package httpapi_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"task260-adnattr/internal/httpapi"
	"task260-adnattr/internal/service"
	"task260-adnattr/internal/store"
)

func TestRequestBodyRejectsTrailingJSON(t *testing.T) {
	st, err := store.OpenStore(t.TempDir() + "/json.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	svc := service.New(st)
	req := httptest.NewRequest("POST", "/api/libraries", strings.NewReader(`{"name":"one"} {"name":"two"}`))
	resp := httptest.NewRecorder()
	httpapi.New(svc).Handler().ServeHTTP(resp, req)
	if resp.Code != 400 {
		t.Fatalf("status = %d, want 400", resp.Code)
	}
	libraries, err := st.ListLibraries()
	if err != nil {
		t.Fatal(err)
	}
	if len(libraries) != 0 {
		t.Fatalf("libraries after malformed request = %d, want 0", len(libraries))
	}
}
