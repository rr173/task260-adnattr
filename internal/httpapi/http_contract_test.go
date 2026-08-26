package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
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

// TestHTTPRejectsExtraJSONObjects 锁定：请求体必须恰好包含一个合法 JSON 对象。
// 客户端发送两个连续 JSON 对象时，接口应返回 400 参数错误，且不产生任何业务写入。
func TestHTTPRejectsExtraJSONObjects(t *testing.T) {
	st, err := store.OpenStore(t.TempDir() + "/http.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	svc := service.New(st)

	h := httpapi.New(svc).Handler()

	cases := []struct {
		name string
		body string
	}{
		{"two objects", `{"name":"alpha"}{"name":"beta"}`},
		{"object plus junk", `{"name":"alpha"}garbage`},
		{"two empty objects", `{}{}`},
		{"object with trailing value", `{"name":"alpha"}42`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/libraries", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			h.ServeHTTP(resp, req)

			if resp.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d for body %q", resp.Code, http.StatusBadRequest, tc.body)
			}
		})
	}

	// 任何带额外内容的请求都不应产生业务写入：全部失败后文库列表应为空。
	req := httptest.NewRequest("GET", "/api/libraries", nil)
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/libraries status = %d, want 200", resp.Code)
	}
	if strings.Contains(resp.Body.String(), `"name"`) {
		t.Fatalf("expected no libraries created, got body: %s", resp.Body.String())
	}
}

// TestHTTPAcceptsSingleJSONObject 确保合法的单对象请求仍正常工作，未被新校验误伤。
func TestHTTPAcceptsSingleJSONObject(t *testing.T) {
	st, err := store.OpenStore(t.TempDir() + "/http.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	svc := service.New(st)

	h := httpapi.New(svc).Handler()
	req := httptest.NewRequest("POST", "/api/libraries", strings.NewReader(`{"name":"alpha","note":""}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusCreated)
	}
}

