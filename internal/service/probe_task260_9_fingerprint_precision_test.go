package service

import "testing"

func TestFingerprintPreservesMetricPrecision(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	lib, err := svc.CreateLibrary("precision", "")
	if err != nil {
		t.Fatal(err)
	}
	_, added, err := svc.IngestFragment(lib.ID, 60, 0.12341, 0.01001, 0.00101, "")
	if err != nil || !added {
		t.Fatalf("first import: added=%v err=%v", added, err)
	}
	_, added, err = svc.IngestFragment(lib.ID, 60, 0.12344, 0.01001, 0.00101, "")
	if err != nil || !added {
		t.Fatalf("second import: added=%v err=%v", added, err)
	}
	frags, err := svc.Store.ListFragmentsByLibrary(lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(frags) != 2 {
		t.Fatalf("stored fragments = %d, want 2", len(frags))
	}
}
