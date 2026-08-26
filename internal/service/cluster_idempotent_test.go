package service

import "testing"

// TestClusterIsIdempotent 复现：对同一文库重复执行聚类应复用已有簇，
// 而非产生重复簇——否则片段簇列表、快照 cluster_counts 与 /api/stats 簇数会不断增长。
func TestClusterIsIdempotent(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	lib, err := svc.CreateLibrary("dup", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []struct {
		length int
		c2t    float64
		g2a    float64
	}{
		{50, 0.22, 0.13},
		{70, 0.18, 0.10},
	} {
		if _, _, err := svc.IngestFragment(lib.ID, f.length, f.c2t, f.g2a, 0.004, ""); err != nil {
			t.Fatal(err)
		}
	}

	first, err := svc.Cluster(lib.ID)
	if err != nil {
		t.Fatalf("first cluster: %v", err)
	}
	want := len(first)
	if want == 0 {
		t.Fatal("expected at least one cluster")
	}

	// 重复执行三次，簇数应保持不变。
	for i := 0; i < 3; i++ {
		got, err := svc.Cluster(lib.ID)
		if err != nil {
			t.Fatalf("repeat cluster #%d: %v", i, err)
		}
		if len(got) != want {
			t.Fatalf("repeat cluster #%d count = %d, want %d (duplicates produced)", i, len(got), want)
		}
	}

	// 库表层面也不应有重复簇。
	var dbCount int
	if err := svc.Store.DB().QueryRow(
		`SELECT COUNT(*) FROM fragment_clusters WHERE library_id = ?`, lib.ID).Scan(&dbCount); err != nil {
		t.Fatal(err)
	}
	if dbCount != want {
		t.Fatalf("db cluster count = %d, want %d (persisted duplicates)", dbCount, want)
	}
}
