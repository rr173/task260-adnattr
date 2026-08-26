package service

import "testing"

func TestSnapshotRejectsUnknownControlReference(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	control, err := svc.CreateControl("blank", true, 150, 0.01, 0.01, 0)
	if err != nil {
		t.Fatal(err)
	}
	lib, err := svc.CreateLibrary("snapshot-control", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.IngestFragment(lib.ID, 60, 0.2, 0.15, 0.01, ""); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Analyze(lib.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PublishSnapshot(lib.ID, control.ID+9999); err == nil {
		t.Fatal("snapshot published with unknown control")
	}
	snapshots, err := svc.Store.ListSnapshotsByLibrary(lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 0 {
		t.Fatalf("snapshot count = %d, want 0", len(snapshots))
	}
}
