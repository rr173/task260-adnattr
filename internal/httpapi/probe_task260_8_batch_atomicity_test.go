package httpapi_test

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"task260-adnattr/internal/httpapi"
	"task260-adnattr/internal/service"
	"task260-adnattr/internal/store"
)

func TestBatchImportRollsBackOnInvalidItem(t *testing.T) {
	st, err := store.OpenStore(t.TempDir() + "/batch.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	svc := service.New(st)
	lib, err := svc.CreateLibrary("batch", "")
	if err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"library_id":%d,"items":[{"frag_len":60,"c2t_5p":0.2,"g2a_3p":0.1,"mean_base_error":0.01},{"frag_len":60,"c2t_5p":0.2,"g2a_3p":0.1,"mean_base_error":0.01,"sequence":"ACGTN"}]}`, lib.ID)
	req := httptest.NewRequest("POST", "/api/fragments/batch", strings.NewReader(body))
	resp := httptest.NewRecorder()
	httpapi.New(svc).Handler().ServeHTTP(resp, req)
	if resp.Code != 400 {
		t.Fatalf("batch status = %d, want 400", resp.Code)
	}
	frags, err := st.ListFragmentsByLibrary(lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(frags) != 0 {
		t.Fatalf("fragments after failed batch = %d, want 0", len(frags))
	}
}
