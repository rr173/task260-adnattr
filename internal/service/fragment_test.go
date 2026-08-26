package service

import (
	"errors"
	"testing"

	"task260-adnattr/internal/model"
)

func TestBatchFragmentsProduceStableProfile(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	lib, err := svc.CreateLibrary("profile", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []struct {
		length int
		c2t    float64
		g2a    float64
	}{
		{40, 0.10, 0.05},
		{60, 0.20, 0.15},
	} {
		if _, _, err := svc.IngestFragment(lib.ID, f.length, f.c2t, f.g2a, 0.01, ""); err != nil {
			t.Fatal(err)
		}
	}
	profile, err := svc.Store.DB().Query("SELECT COUNT(*) FROM fragment_summaries WHERE library_id = ?", lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer profile.Close()
	if !profile.Next() {
		t.Fatal("expected aggregate row")
	}
	var count int
	if err := profile.Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("fragment count = %d, want 2", count)
	}
}

// TestIngestFragmentsAtomicLeavesNoPartialRows 验证批量导入的原子性：
// 整批中只要有一条数据非法（非法碱基），就不应向文库写入任何片段。
func TestIngestFragmentsAtomicLeavesNoPartialRows(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	lib, err := svc.CreateLibrary("L", "")
	if err != nil {
		t.Fatalf("create library: %v", err)
	}
	// 前两条合法，最后一条包含非法碱基 N。
	inputs := []FragmentInput{
		{FragLen: 40, C2T5p: 0.10, G2A3p: 0.05, MeanBaseError: 0.01, Sequence: "ACGTACGT"},
		{FragLen: 60, C2T5p: 0.20, G2A3p: 0.15, MeanBaseError: 0.01, Sequence: "ACGTACGTAC"},
		{FragLen: 0, C2T5p: 0.0, G2A3p: 0.0, MeanBaseError: 0.01, Sequence: "ACGTN"},
	}
	added, ignored, err := svc.IngestFragmentsAtomic(lib.ID, inputs)
	if err == nil {
		t.Fatal("expected error for invalid base, got nil")
	}
	if !errors.Is(err, model.ErrInvalidBase) {
		t.Fatalf("expected ErrInvalidBase, got %v", err)
	}
	if added != 0 || ignored != 0 {
		t.Fatalf("added=%d ignored=%d, want 0/0 on failed batch", added, ignored)
	}
	var count int
	row := svc.Store.DB().QueryRow("SELECT COUNT(*) FROM fragment_summaries WHERE library_id = ?", lib.ID)
	if err := row.Scan(&count); err != nil {
		t.Fatalf("scan count: %v", err)
	}
	if count != 0 {
		t.Fatalf("fragment count = %d, want 0 (batch must be atomic, no partial writes)", count)
	}
}

// TestIngestFragmentsAtomicCommitsAllOnSuccess 验证整批合法时全部写入并返回计数。
func TestIngestFragmentsAtomicCommitsAllOnSuccess(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	lib, err := svc.CreateLibrary("L", "")
	if err != nil {
		t.Fatalf("create library: %v", err)
	}
	inputs := []FragmentInput{
		{FragLen: 40, C2T5p: 0.10, G2A3p: 0.05, MeanBaseError: 0.01, Sequence: "ACGTACGT"},
		{FragLen: 60, C2T5p: 0.20, G2A3p: 0.15, MeanBaseError: 0.01, Sequence: "ACGTACGTAC"},
	}
	added, ignored, err := svc.IngestFragmentsAtomic(lib.ID, inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if added != 2 || ignored != 0 {
		t.Fatalf("added=%d ignored=%d, want 2/0", added, ignored)
	}
	var count int
	row := svc.Store.DB().QueryRow("SELECT COUNT(*) FROM fragment_summaries WHERE library_id = ?", lib.ID)
	if err := row.Scan(&count); err != nil {
		t.Fatalf("scan count: %v", err)
	}
	if count != 2 {
		t.Fatalf("fragment count = %d, want 2", count)
	}
}
