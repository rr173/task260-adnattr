package service

import (
	"testing"

	"task260-adnattr/internal/model"
)

func TestControlAssociationIsIdempotent(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	lib, err := svc.CreateLibrary("library", "")
	if err != nil {
		t.Fatal(err)
	}
	control, err := svc.CreateControl("blank", true, 150, 0.01, 0.01, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.AssociateControl(lib.ID, control.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.AssociateControl(lib.ID, control.ID); err != nil {
		t.Fatal(err)
	}
	linked, err := svc.Store.GetLinkedControls(lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(linked) != 1 || linked[0].ID != control.ID {
		t.Fatalf("linked controls = %#v, want one control %d", linked, control.ID)
	}
}

func TestSelfCheckReportsHealthyScoring(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	result, err := svc.SelfCheck()
	if err != nil {
		t.Fatal(err)
	}
	if result["db"] != "ok" || result["scoring"] != model.AttribDegradation {
		t.Fatalf("self-check result = %#v", result)
	}
}
