package service

import (
	"errors"
	"path/filepath"
	"testing"

	"task260-adnattr/internal/model"
	"task260-adnattr/internal/store"
)

// newTestService 打开临时库并返回 Service 与清理函数。
func newTestService(t *testing.T) (*Service, func()) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	st, err := store.OpenStore(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return New(st), func() { _ = st.Close() }
}

func TestAnalyzeModernContamination(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	if _, err := svc.CreateControl("blank", true, 150, 0.01, 0.01, 0); err != nil {
		t.Fatalf("create blank: %v", err)
	}
	lib, err := svc.CreateLibrary("B", "screening")
	if err != nil {
		t.Fatalf("create library: %v", err)
	}
	// 长片段、脱氨低，与空白一致 → 现代污染。
	frags := []struct {
		len int
		c2t float64
		g2a float64
	}{{160, 0.02, 0.01}, {180, 0.015, 0.012}, {200, 0.01, 0.008}}
	for _, f := range frags {
		if _, _, err := svc.IngestFragment(lib.ID, f.len, f.c2t, f.g2a, 0.005, ""); err != nil {
			t.Fatalf("ingest: %v", err)
		}
	}
	if _, err := svc.Cluster(lib.ID); err != nil {
		t.Fatalf("cluster: %v", err)
	}
	_, cand, err := svc.Analyze(lib.ID)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if cand.Kind != model.AttribModernContamination {
		t.Fatalf("kind = %s, want %s", cand.Kind, model.AttribModernContamination)
	}
}

func TestAnalyzeAuthenticDegradation(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	if _, err := svc.CreateControl("blank", true, 150, 0.01, 0.01, 0); err != nil {
		t.Fatalf("create blank: %v", err)
	}
	lib, err := svc.CreateLibrary("A", "ancient")
	if err != nil {
		t.Fatalf("create library: %v", err)
	}
	frags := []struct {
		len int
		c2t float64
		g2a float64
	}{{50, 0.22, 0.13}, {65, 0.19, 0.11}, {75, 0.24, 0.14}}
	for _, f := range frags {
		if _, _, err := svc.IngestFragment(lib.ID, f.len, f.c2t, f.g2a, 0.004, ""); err != nil {
			t.Fatalf("ingest: %v", err)
		}
	}
	if _, err := svc.Cluster(lib.ID); err != nil {
		t.Fatalf("cluster: %v", err)
	}
	_, cand, err := svc.Analyze(lib.ID)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if cand.Kind != model.AttribDegradation {
		t.Fatalf("kind = %s, want %s", cand.Kind, model.AttribDegradation)
	}
}

func TestIngestIdempotentByFingerprint(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	lib, err := svc.CreateLibrary("L", "")
	if err != nil {
		t.Fatalf("create library: %v", err)
	}
	_, added1, err := svc.IngestFragment(lib.ID, 60, 0.2, 0.1, 0.005, "ACGTACGT")
	if err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	if !added1 {
		t.Fatal("first ingest should be added")
	}
	_, added2, err := svc.IngestFragment(lib.ID, 60, 0.2, 0.1, 0.005, "ACGTACGT")
	if err != nil {
		t.Fatalf("second ingest: %v", err)
	}
	if added2 {
		t.Fatal("duplicate ingest should be ignored (idempotent)")
	}
}

func TestIngestRejectsInvalidBase(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	lib, err := svc.CreateLibrary("L", "")
	if err != nil {
		t.Fatalf("create library: %v", err)
	}
	if _, _, err := svc.IngestFragment(lib.ID, 0, 0, 0, 0, "ACGTN"); err == nil {
		t.Fatal("expected ErrInvalidBase for sequence containing N")
	} else if !errors.Is(err, model.ErrInvalidBase) {
		t.Fatalf("err = %v, want ErrInvalidBase", err)
	}
}

func TestSealedLibraryRejectsMutation(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	lib, err := svc.CreateLibrary("L", "")
	if err != nil {
		t.Fatalf("create library: %v", err)
	}
	if _, err := svc.AdvanceLibrary(lib.ID, model.LibPendingAnalysis); err != nil {
		t.Fatalf("advance: %v", err)
	}
	if _, err := svc.AdvanceLibrary(lib.ID, model.LibNeedsReview); err != nil {
		t.Fatalf("advance: %v", err)
	}
	if _, err := svc.AdvanceLibrary(lib.ID, model.LibPublished); err != nil {
		t.Fatalf("advance: %v", err)
	}
	if _, err := svc.SealLibrary(lib.ID); err != nil {
		t.Fatalf("seal: %v", err)
	}
	if _, _, err := svc.IngestFragment(lib.ID, 50, 0.2, 0.1, 0.005, ""); !errors.Is(err, model.ErrSealed) {
		t.Fatalf("expected ErrSealed, got %v", err)
	}
}

// TestSealedLibraryRejectsAnalysis 锁定封存不可变：封存后再次发起分析请求必须被拒绝，
// 且不得新增或覆盖损伤轮廓与归因记录（封存数据保持不变）。
func TestSealedLibraryRejectsAnalysis(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	if _, err := svc.CreateControl("blank", true, 150, 0.01, 0.01, 0); err != nil {
		t.Fatalf("create blank: %v", err)
	}
	lib, err := svc.CreateLibrary("sealed", "")
	if err != nil {
		t.Fatalf("create library: %v", err)
	}
	if _, _, err := svc.IngestFragment(lib.ID, 60, 0.2, 0.1, 0.004, ""); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if _, err := svc.Cluster(lib.ID); err != nil {
		t.Fatalf("cluster: %v", err)
	}
	prof, cand, err := svc.Analyze(lib.ID)
	if err != nil {
		t.Fatalf("first analyze: %v", err)
	}
	// 推进到 published 后封存。
	if _, err := svc.AdvanceLibrary(lib.ID, model.LibPendingAnalysis); err != nil {
		t.Fatalf("advance to pending_analysis: %v", err)
	}
	if _, err := svc.AdvanceLibrary(lib.ID, model.LibNeedsReview); err != nil {
		t.Fatalf("advance to needs_review: %v", err)
	}
	if _, err := svc.AdvanceLibrary(lib.ID, model.LibPublished); err != nil {
		t.Fatalf("advance to published: %v", err)
	}
	if _, err := svc.SealLibrary(lib.ID); err != nil {
		t.Fatalf("seal: %v", err)
	}

	// 封存后再次分析必须被拒绝。
	if _, _, err := svc.Analyze(lib.ID); !errors.Is(err, model.ErrSealed) {
		t.Fatalf("analyze sealed library: got %v, want ErrSealed", err)
	}

	// 损伤轮廓未被覆盖。
	profAfter, err := svc.Store.GetDamageProfile(lib.ID)
	if err != nil {
		t.Fatalf("get damage after: %v", err)
	}
	if profAfter.ComputedAt != prof.ComputedAt || profAfter.Deam5p != prof.Deam5p {
		t.Fatalf("sealed damage profile mutated: computed_at %d->%d deam5p %v->%v",
			prof.ComputedAt, profAfter.ComputedAt, prof.Deam5p, profAfter.Deam5p)
	}
	// 归因记录未被删除或新增。
	attrs, err := svc.Store.ListAttributionsByLibrary(lib.ID)
	if err != nil {
		t.Fatalf("list attributions: %v", err)
	}
	if len(attrs) != 1 || attrs[0].ID != cand.ID {
		t.Fatalf("sealed attributions mutated: count=%d, want 1 (id=%d)", len(attrs), cand.ID)
	}
}

