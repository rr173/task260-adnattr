package service

import (
	"errors"
	"testing"

	"task260-adnattr/internal/model"
)

func TestSealedLibraryRejectsClusterMutation(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	lib, err := svc.CreateLibrary("sealed-cluster", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.IngestFragment(lib.ID, 60, 0.2, 0.15, 0.01, ""); err != nil {
		t.Fatal(err)
	}
	clusters, err := svc.Cluster(lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{model.LibPendingAnalysis, model.LibNeedsReview, model.LibPublished} {
		if _, err := svc.AdvanceLibrary(lib.ID, status); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.SealLibrary(lib.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ClassifyCluster(clusters[0].ID, model.FragDamageConsistent); !errors.Is(err, model.ErrSealed) {
		t.Fatalf("classify sealed cluster: got %v", err)
	}
	cluster, err := svc.Store.GetCluster(clusters[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if cluster.Status != model.FragRaw {
		t.Fatalf("sealed cluster status = %s, want raw", cluster.Status)
	}
}
