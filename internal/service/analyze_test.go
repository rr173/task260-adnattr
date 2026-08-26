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

// TestAnalyzeSingleEndDeamIsNotDegradation 覆盖回归：仅 5' 端 C→T 富集、3' 端 G→A 缺失，
// 即便片段偏短，也不得归为真实降解——两端损伤轮廓与短片段须共同满足。
func TestAnalyzeSingleEndDeamIsNotDegradation(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	if _, err := svc.CreateControl("blank", true, 150, 0.01, 0.01, 0); err != nil {
		t.Fatalf("create blank: %v", err)
	}
	lib, err := svc.CreateLibrary("C", "single-end deam only")
	if err != nil {
		t.Fatalf("create library: %v", err)
	}
	// 短片段 + 5' 端 C→T 高，但 3' 端 G→A 接近空白水平。
	frags := []struct {
		len int
		c2t float64
		g2a float64
	}{{50, 0.22, 0.01}, {65, 0.19, 0.012}, {75, 0.24, 0.008}}
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
	if cand.Kind == model.AttribDegradation {
		t.Fatalf("single-end deam must not be degradation, got %s (reason=%s)", cand.Kind, cand.Reason)
	}
	// 片段簇亦不应被自动归为 damage_consistent。
	clusters, err := svc.Store.ListClustersByLibrary(lib.ID)
	if err != nil {
		t.Fatalf("list clusters: %v", err)
	}
	for _, c := range clusters {
		if c.Status == model.FragDamageConsistent {
			t.Fatalf("cluster %d status = %s, want not damage_consistent (single-end deam)", c.ID, c.Status)
		}
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
