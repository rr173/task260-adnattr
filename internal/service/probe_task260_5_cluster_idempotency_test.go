package service

import "testing"

func TestRepeatedClusteringIsIdempotent(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	lib, err := svc.CreateLibrary("cluster-idempotency", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.IngestFragment(lib.ID, 60, 0.2, 0.15, 0.01, ""); err != nil {
		t.Fatal(err)
	}
	first, err := svc.Cluster(lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Cluster(lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || len(second) != 1 || first[0].ID != second[0].ID {
		t.Fatalf("first=%v second=%v, want the same single cluster", first, second)
	}
	all, err := svc.Store.ListClustersByLibrary(lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("stored clusters = %d, want 1", len(all))
	}
}
