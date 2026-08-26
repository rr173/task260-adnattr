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

func TestAnalyzePrefersLinkedBlankOverGlobalPool(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	lib, err := svc.CreateLibrary("L", "")
	if err != nil {
		t.Fatalf("create library: %v", err)
	}
	// 全局空白池：与研究者的预期参照截然不同（差异巨大），若被选用会改变归因分。
	globalBlank, err := svc.CreateControl("global-blank", true, 60, 0.30, 0.30, 0)
	if err != nil {
		t.Fatalf("create global blank: %v", err)
	}
	// 研究者为本文库明确选定的参照：长片段、脱氨低，与该文库片段一致 → 现代污染。
	linkedBlank, err := svc.CreateControl("linked-blank", true, 160, 0.01, 0.01, 0)
	if err != nil {
		t.Fatalf("create linked blank: %v", err)
	}
	if err := svc.AssociateControl(lib.ID, linkedBlank.ID); err != nil {
		t.Fatalf("associate linked blank: %v", err)
	}
	// 现代污染风格片段：长、脱氨弱，与 linked-blank 高度相似。
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
	// 若分析误用全局空白（差异大的 globalBlank），相似度会很低、阈值不达 modern_contamination。
	// 只有在使用研究者选定的 linkedBlank 时，才会被判为现代污染。
	if cand.Kind != model.AttribModernContamination {
		t.Fatalf("kind = %s, want %s (analysis must use the researcher-linked blank, not the global pool)",
			cand.Kind, model.AttribModernContamination)
	}
	// 全局空白仍存在于系统中，证明筛选生效而非依赖空池。
	blanks, err := svc.Store.ListBlankControls()
	if err != nil {
		t.Fatalf("list blanks: %v", err)
	}
	if len(blanks) < 2 || blanks[0].ID != globalBlank.ID {
		t.Fatalf("global pool should still contain both blanks, got %#v", blanks)
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
