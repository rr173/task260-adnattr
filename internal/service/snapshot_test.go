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

// TestPublishSnapshotRejectsNonExistentControl 断言：发布时填入不存在的对照编号应被拒绝，
// 不得产出引用无法追溯对照批次的已发布快照。
func TestPublishSnapshotRejectsNonExistentControl(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	lib, err := svc.CreateLibrary("snapshot-missing", "")
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

	// 不存在的对照编号：应返回 ErrUnknownControl，且不得写入任何已发布快照。
	missingID := control.ID + 9999
	if _, err := svc.PublishSnapshot(lib.ID, missingID); !errors.Is(err, model.ErrUnknownControl) {
		t.Fatalf("publish with non-existent control: got %v, want ErrUnknownControl", err)
	}
	snaps, err := svc.Store.ListSnapshotsByLibrary(lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range snaps {
		if s.Status == model.SnapPublished {
			t.Fatalf("no published snapshot expected for rejected control, got status=%s control=%d", s.Status, s.ControlBatchID)
		}
	}
}

// TestPublishSnapshotRejectsNonBlankControl 断言：发布时只能冻结空白对照，
// 非空白参考对照应被拒绝（ErrControlMissing），避免冻结不符合空白对照要求的参照。
func TestPublishSnapshotRejectsNonBlankControl(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	lib, err := svc.CreateLibrary("snapshot-nonblank", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.IngestFragment(lib.ID, 60, 0.2, 0.1, 0.01, ""); err != nil {
		t.Fatal(err)
	}
	// 保证存在空白对照使 Analyze 通过。
	if _, err := svc.CreateControl("blank", true, 150, 0.01, 0.01, 0); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Analyze(lib.ID); err != nil {
		t.Fatal(err)
	}
	// 非空白参考对照。
	reference, err := svc.CreateControl("ref-sample", false, 150, 0.05, 0.04, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PublishSnapshot(lib.ID, reference.ID); !errors.Is(err, model.ErrControlMissing) {
		t.Fatalf("publish with non-blank control: got %v, want ErrControlMissing", err)
	}
	snaps, err := svc.Store.ListSnapshotsByLibrary(lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range snaps {
		if s.Status == model.SnapPublished {
			t.Fatalf("no published snapshot expected for non-blank control, got status=%s control=%d", s.Status, s.ControlBatchID)
		}
	}
}
