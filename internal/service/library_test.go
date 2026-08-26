package service

import (
	"errors"
	"testing"

	"task260-adnattr/internal/model"
)

func TestLibraryLifecycleRejectsInvalidTransitions(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	lib, err := svc.CreateLibrary("lifecycle", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AdvanceLibrary(lib.ID, model.LibPublished); !errors.Is(err, model.ErrInvalidStatus) {
		t.Fatalf("advance receiving directly to published: got %v", err)
	}
	if _, err := svc.AdvanceLibrary(lib.ID, model.LibPendingAnalysis); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AdvanceLibrary(lib.ID, model.LibNeedsReview); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AdvanceLibrary(lib.ID, model.LibPublished); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SealLibrary(lib.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AdvanceLibrary(lib.ID, model.LibPublished); !errors.Is(err, model.ErrSealed) {
		t.Fatalf("advance sealed library: got %v", err)
	}
}
