package service

import (
	"testing"

	"task260-adnattr/internal/model"
)

func TestSingleEndedDamageIsNotDamageConsistent(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	if _, err := svc.CreateControl("blank", true, 150, 0.01, 0.01, 0); err != nil {
		t.Fatal(err)
	}
	lib, err := svc.CreateLibrary("single-ended", "")
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if _, _, err := svc.IngestFragment(lib.ID, 60, 0.20, 0.01, 0.01, ""); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.Cluster(lib.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Analyze(lib.ID); err != nil {
		t.Fatal(err)
	}
	clusters, err := svc.Store.ListClustersByLibrary(lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(clusters) != 1 || clusters[0].Status != model.FragLowQuality {
		t.Fatalf("clusters = %#v, want one low_quality cluster", clusters)
	}
}
