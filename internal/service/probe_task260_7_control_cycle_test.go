package service

import (
	"errors"
	"testing"

	"task260-adnattr/internal/model"
)

func TestControlAssociationRejectsCycle(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	a, err := svc.CreateLibrary("A", "")
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.CreateLibrary("B", "")
	if err != nil {
		t.Fatal(err)
	}
	fromB, err := svc.CreateControl("from-B", false, 150, 0.01, 0.01, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	fromA, err := svc.CreateControl("from-A", false, 150, 0.01, 0.01, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.AssociateControl(a.ID, fromB.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.AssociateControl(b.ID, fromA.ID); !errors.Is(err, model.ErrBatchCycle) {
		t.Fatalf("cycle association error = %v, want ErrBatchCycle", err)
	}
}
