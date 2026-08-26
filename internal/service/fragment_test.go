package service

import "testing"

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

// TestFingerprintPreservesFullPrecision 验证指纹保留输入统计值的全部有效精度：
// 仅末位小数不同的两条摘要不得被当成同一条片段而相互吞并，仍应各写一条。
func TestFingerprintPreservesFullPrecision(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	lib, err := svc.CreateLibrary("precision", "")
	if err != nil {
		t.Fatal(err)
	}
	// 仅在平均错误率末位小数上不同：0.0040 vs 0.0041。
	if _, added1, err := svc.IngestFragment(lib.ID, 60, 0.20, 0.10, 0.0040, ""); err != nil {
		t.Fatalf("first ingest: %v", err)
	} else if !added1 {
		t.Fatal("first ingest should be added")
	}
	if _, added2, err := svc.IngestFragment(lib.ID, 60, 0.20, 0.10, 0.0041, ""); err != nil {
		t.Fatalf("second ingest: %v", err)
	} else if !added2 {
		t.Fatal("second ingest (differs only in last decimal of mean_base_error) should be added, not deduplicated")
	}
	// 真正相同的摘要仍应幂等忽略。
	if _, added3, err := svc.IngestFragment(lib.ID, 60, 0.20, 0.10, 0.0041, ""); err != nil {
		t.Fatalf("third ingest: %v", err)
	} else if added3 {
		t.Fatal("exact duplicate should be ignored (idempotent)")
	}
	row := svc.Store.DB().QueryRow("SELECT COUNT(*) FROM fragment_summaries WHERE library_id = ?", lib.ID)
	var count int
	if err := row.Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("fragment count = %d, want 2 distinct summaries", count)
	}
}
