package service

import (
	"errors"
	"testing"

	"task260-adnattr/internal/model"
)

func TestSnapshotPublishedCanOnlyBeSuperseded(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	lib, err := svc.CreateLibrary("snapshot", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.IngestFragment(lib.ID, 60, 0.2, 0.1, 0.01, ""); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Analyze(lib.ID); !errors.Is(err, model.ErrControlMissing) {
		t.Fatalf("analyze without blank: got %v", err)
	}
	control, err := svc.CreateControl("blank", true, 150, 0.01, 0.01, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Analyze(lib.ID); err != nil {
		t.Fatal(err)
	}
	snap, err := svc.PublishSnapshot(lib.ID, control.ID)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Status != model.SnapPublished {
		t.Fatalf("snapshot status = %s", snap.Status)
	}
	if _, err := svc.SupersedeSnapshot(snap.ID); err != nil {
		t.Fatal(err)
	}
}
