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

// TestClassifyClusterRejectedAfterSeal 验证封存文库下的片段簇保持只读：
// 通过片段簇裁决接口修改归类状态应被拒绝。
func TestClassifyClusterRejectedAfterSeal(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	lib, err := svc.CreateLibrary("sealed", "")
	if err != nil {
		t.Fatalf("create library: %v", err)
	}
	if _, _, err := svc.IngestFragment(lib.ID, 60, 0.2, 0.1, 0.005, ""); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if _, err := svc.Cluster(lib.ID); err != nil {
		t.Fatalf("cluster: %v", err)
	}
	clusters, err := svc.Store.ListClustersByLibrary(lib.ID)
	if err != nil {
		t.Fatalf("list clusters: %v", err)
	}
	if len(clusters) != 1 {
		t.Fatalf("cluster count = %d, want 1", len(clusters))
	}
	c := clusters[0]
	if _, err := svc.AdvanceLibrary(lib.ID, model.LibPendingAnalysis); err != nil {
		t.Fatalf("advance to pending: %v", err)
	}
	if _, err := svc.AdvanceLibrary(lib.ID, model.LibNeedsReview); err != nil {
		t.Fatalf("advance to review: %v", err)
	}
	if _, err := svc.AdvanceLibrary(lib.ID, model.LibPublished); err != nil {
		t.Fatalf("advance to published: %v", err)
	}
	if _, err := svc.SealLibrary(lib.ID); err != nil {
		t.Fatalf("seal: %v", err)
	}
	if _, err := svc.ClassifyCluster(c.ID, model.FragDamageConsistent); !errors.Is(err, model.ErrSealed) {
		t.Fatalf("classify sealed cluster: got %v, want ErrSealed", err)
	}
}
