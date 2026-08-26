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
