package service

import (
	"errors"
	"testing"

	"task260-adnattr/internal/model"
)

func TestSealedLibraryRejectsAnalysisWrites(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	lib, err := svc.CreateLibrary("sealed-analysis", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AdvanceLibrary(lib.ID, model.LibPendingAnalysis); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.IngestFragment(lib.ID, 60, 0.2, 0.15, 0.01, ""); err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{model.LibNeedsReview, model.LibPublished} {
		if _, err := svc.AdvanceLibrary(lib.ID, status); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.SealLibrary(lib.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Analyze(lib.ID); !errors.Is(err, model.ErrSealed) {
		t.Fatalf("analyze sealed library: got %v", err)
	}
	if _, err := svc.Store.GetDamageProfile(lib.ID); err == nil {
		t.Fatal("sealed analysis created a damage profile")
	}
	attrs, err := svc.Store.ListAttributionsByLibrary(lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(attrs) != 0 {
		t.Fatalf("sealed analysis created %d attributions", len(attrs))
	}
}
