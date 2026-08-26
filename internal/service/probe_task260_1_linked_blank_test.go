package service

import (
	"testing"

	"task260-adnattr/internal/model"
)

func TestLinkedBlankControlsOverrideGlobalFallback(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	global, err := svc.CreateControl("global", true, 300, 0.40, 0.40, 0)
	if err != nil {
		t.Fatal(err)
	}
	_ = global
	selected, err := svc.CreateControl("selected", true, 130, 0.06, 0.06, 0)
	if err != nil {
		t.Fatal(err)
	}
	lib, err := svc.CreateLibrary("linked", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.AssociateControl(lib.ID, selected.ID); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if _, _, err := svc.IngestFragment(lib.ID, 130, 0.06, 0.06, 0.01, ""); err != nil {
			t.Fatal(err)
		}
	}
	_, candidate, err := svc.Analyze(lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	if candidate.Kind != model.AttribModernContamination {
		t.Fatalf("candidate kind = %s, want %s", candidate.Kind, model.AttribModernContamination)
	}
}
