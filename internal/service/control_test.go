package service

import (
	"errors"
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

// TestControlLinkRejectsSelfReference 校验：把某文库产生的对照再关联回该文库
// 本身，构成自引用，应被拒绝。
func TestControlLinkRejectsSelfReference(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	lib, err := svc.CreateLibrary("self-ref-lib", "")
	if err != nil {
		t.Fatal(err)
	}
	control, err := svc.CreateControl("derived-from-lib", false, 150, 0.01, 0.01, lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.AssociateControl(lib.ID, control.ID); !errors.Is(err, model.ErrSelfReference) {
		t.Fatalf("associate self-derived control: got %v, want ErrSelfReference", err)
	}
}

// TestControlLinkRejectsCrossLibraryCycle 校验：两个文库互相把对方产生的对照
// 关联为参考，构成跨文库循环；第一次关联（尚无环）允许，第二次关联（闭合环）拒绝。
func TestControlLinkRejectsCrossLibraryCycle(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	libA, err := svc.CreateLibrary("libA", "")
	if err != nil {
		t.Fatal(err)
	}
	libB, err := svc.CreateLibrary("libB", "")
	if err != nil {
		t.Fatal(err)
	}
	// libA 产生的对照，可被 libB 关联（此时无环）。
	fromA, err := svc.CreateControl("control-from-libA", false, 150, 0.01, 0.01, libA.ID)
	if err != nil {
		t.Fatal(err)
	}
	// libB 产生的对照，可被 libA 关联（此时无环）。
	fromB, err := svc.CreateControl("control-from-libB", false, 140, 0.02, 0.01, libB.ID)
	if err != nil {
		t.Fatal(err)
	}

	// 第一次：libB 关联 libA 产生的对照——尚不构成循环，应成功。
	if err := svc.AssociateControl(libB.ID, fromA.ID); err != nil {
		t.Fatalf("associate libB -> fromA (no cycle yet): %v", err)
	}
	// 第二次：libA 关联 libB 产生的对照——此时 libB 已依赖 libA，会闭合循环，应拒绝。
	if err := svc.AssociateControl(libA.ID, fromB.ID); !errors.Is(err, model.ErrBatchCycle) {
		t.Fatalf("associate libA -> fromB (closes cycle): got %v, want ErrBatchCycle", err)
	}

	// 反向同理：libA 已（在上一步被尝试关联前）若先建另一条边再回环也应拒绝。
	// 验证独立无环关联仍正常。
	if err := svc.AssociateControl(libA.ID, fromA.ID); !errors.Is(err, model.ErrSelfReference) {
		t.Fatalf("associate libA -> fromA (self): got %v, want ErrSelfReference", err)
	}
}

// TestControlLinkAllowsIndependentChains 校验：多个文库共享同一来源文库的对照
// 不构成循环时，关联链可以正常建立（避免循环检测过严）。
func TestControlLinkAllowsIndependentChains(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	root, err := svc.CreateLibrary("root-lib", "")
	if err != nil {
		t.Fatal(err)
	}
	c1, err := svc.CreateControl("c-from-root", false, 150, 0.01, 0.01, root.ID)
	if err != nil {
		t.Fatal(err)
	}
	leaf1, err := svc.CreateLibrary("leaf1", "")
	if err != nil {
		t.Fatal(err)
	}
	leaf2, err := svc.CreateLibrary("leaf2", "")
	if err != nil {
		t.Fatal(err)
	}
	// 两个不同文库各自关联 root 产生的对照：DAG，无环，均应成功。
	if err := svc.AssociateControl(leaf1.ID, c1.ID); err != nil {
		t.Fatalf("associate leaf1 -> c1: %v", err)
	}
	if err := svc.AssociateControl(leaf2.ID, c1.ID); err != nil {
		t.Fatalf("associate leaf2 -> c1: %v", err)
	}
	linked, err := svc.Store.GetLinkedControls(leaf1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(linked) != 1 || linked[0].ID != c1.ID {
		t.Fatalf("leaf1 linked = %#v, want c1=%d", linked, c1.ID)
	}
}
