// adnattr 古 DNA 片段损伤与污染归因服务入口。
//
// 用法：
//
//	adnattr --addr :8080 --db ./adnattr.db     启动 HTTP 服务
//	adnattr --smoke-test                       自检：真实建库→分析闭环→关闭重开验证恢复→退出 0
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"task260-adnattr/internal/httpapi"
	"task260-adnattr/internal/model"
	"task260-adnattr/internal/service"
	"task260-adnattr/internal/store"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	dbPath := flag.String("db", "./adnattr.db", "sqlite database path")
	smoke := flag.Bool("smoke-test", false, "run closed-loop self test and exit 0 on success")
	flag.Parse()

	if *smoke {
		if err := runSmokeTest(*dbPath); err != nil {
			fmt.Fprintf(os.Stderr, "smoke-test FAILED: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("smoke-test OK")
		return
	}

	st, err := store.OpenStore(*dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()
	svc := service.New(st)
	srv := &http.Server{Addr: *addr, Handler: httpapi.New(svc).Handler()}
	log.Printf("adnattr listening on %s (db=%s)", *addr, *dbPath)
	log.Fatal(srv.ListenAndServe())
}

// smokeCheck 断言辅助：条件不满足时返回错误并中止自检。
func smokeCheck(cond bool, format string, args ...interface{}) error {
	if !cond {
		return fmt.Errorf(format, args...)
	}
	return nil
}

// runSmokeTest 执行真实闭环自检：
//  1. 真实建库、建文库、建空白对照、导入片段（指纹幂等）；
//  2. 聚类 → 分析（损伤轮廓 + 污染归因）→ 确认候选；
//  3. 第二条污染文库：归因现代污染 → 排除批次；
//  4. 发布可信度快照；
//  5. 关闭并重新打开同一数据库，验证实体、轮廓、归因、快照全部恢复；
//  6. 全部断言通过返回 nil（退出码 0）。
func runSmokeTest(dbPath string) error {
	dir := filepath.Dir(dbPath)
	if dir == "" || dir == "." {
		dir = "."
	}
	path := filepath.Join(dir, "smoke-adnattr.db")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}

	// ---- 第一轮：写入 ----
	st, err := store.OpenStore(path)
	if err != nil {
		return err
	}
	svc := service.New(st)

	// 1. 空白对照（负对照：无古代信号，长片段、脱氨低）。
	blank, err := svc.CreateControl("blank-extract-neg", true, 150, 0.01, 0.01, 0)
	if err != nil {
		return fmt.Errorf("create blank control: %w", err)
	}

	// 2. 文库 A：典型古 DNA（短片段 + 末端 C→T 脱氨富集）。
	libA, err := svc.CreateLibrary("Ancient Mural A", "authentic ancient signal")
	if err != nil {
		return fmt.Errorf("create libA: %w", err)
	}
	if _, err := svc.AdvanceLibrary(libA.ID, model.LibPendingAnalysis); err != nil {
		return fmt.Errorf("advance libA: %w", err)
	}
	if libA, err = svc.Store.GetLibrary(libA.ID); err != nil {
		return err
	}
	aFrags := []struct {
		len  int
		c2t  float64
		g2a  float64
	}{
		{48, 0.22, 0.13}, {55, 0.19, 0.11}, {62, 0.24, 0.14},
		{71, 0.21, 0.12}, {80, 0.18, 0.10},
	}
	for i, f := range aFrags {
		if _, added, err := svc.IngestFragment(libA.ID, f.len, f.c2t, f.g2a, 0.004, ""); err != nil {
			return fmt.Errorf("ingest A#%d: %w", i, err)
		} else if !added {
			return fmt.Errorf("ingest A#%d: expected added", i)
		}
	}
	// 幂等：重导同一片段应被忽略。
	if _, added, err := svc.IngestFragment(libA.ID, aFrags[0].len, aFrags[0].c2t, aFrags[0].g2a, 0.004, ""); err != nil {
		return err
	} else if added {
		return fmt.Errorf("expected idempotent ignore for duplicate fragment")
	}
	if _, err := svc.Cluster(libA.ID); err != nil {
		return fmt.Errorf("cluster A: %w", err)
	}
	profA, candA, err := svc.Analyze(libA.ID)
	if err != nil {
		return fmt.Errorf("analyze A: %w", err)
	}
	if err := smokeCheck(candA.Kind == model.AttribDegradation,
		"libA attribution kind = %s, want degradation", candA.Kind); err != nil {
		return err
	}
	if err := smokeCheck(profA.Deam5p > 0.08 && profA.MeanLen < 120,
		"libA profile deam5p=%.3f len=%.1f, want ancient-like", profA.Deam5p, profA.MeanLen); err != nil {
		return err
	}
	if _, err := svc.ConfirmAttribution(candA.ID); err != nil {
		return fmt.Errorf("confirm A: %w", err)
	}

	// 3. 文库 B：现代污染（长片段、脱氨低，与空白对照一致）。
	libB, err := svc.CreateLibrary("Contaminated Batch B", "low-coverage screening run")
	if err != nil {
		return fmt.Errorf("create libB: %w", err)
	}
	if _, err := svc.AdvanceLibrary(libB.ID, model.LibPendingAnalysis); err != nil {
		return fmt.Errorf("advance libB: %w", err)
	}
	bFrags := []struct {
		len  int
		c2t  float64
		g2a  float64
	}{
		{145, 0.02, 0.01}, {160, 0.015, 0.012}, {175, 0.01, 0.008},
		{190, 0.025, 0.02}, {210, 0.02, 0.015},
	}
	for i, f := range bFrags {
		if _, _, err := svc.IngestFragment(libB.ID, f.len, f.c2t, f.g2a, 0.006, ""); err != nil {
			return fmt.Errorf("ingest B#%d: %w", i, err)
		}
	}
	if _, err := svc.Cluster(libB.ID); err != nil {
		return fmt.Errorf("cluster B: %w", err)
	}
	_, candB, err := svc.Analyze(libB.ID)
	if err != nil {
		return fmt.Errorf("analyze B: %w", err)
	}
	if err := smokeCheck(candB.Kind == model.AttribModernContamination,
		"libB attribution kind = %s, want modern_contamination", candB.Kind); err != nil {
		return err
	}
	// 排除污染批次：可疑片段簇置 excluded，文库推进到 needs_review。
	lbB, err := svc.ExcludeBatch(libB.ID)
	if err != nil {
		return fmt.Errorf("exclude B: %w", err)
	}
	if err := smokeCheck(lbB.Status == model.LibNeedsReview, "libB status = %s, want needs_review", lbB.Status); err != nil {
		return err
	}
	bClusters, err := svc.Store.ListClustersByLibrary(libB.ID)
	if err != nil {
		return err
	}
	for _, c := range bClusters {
		if err := smokeCheck(c.Status == model.FragExcluded,
			"libB cluster %d status = %s, want excluded", c.ID, c.Status); err != nil {
			return err
		}
	}

	// 4. 发布可信度快照（冻结空白对照）。
	snapA, err := svc.PublishSnapshot(libA.ID, blank.ID)
	if err != nil {
		return fmt.Errorf("publish snapshot A: %w", err)
	}
	if err := smokeCheck(snapA.Status == model.SnapPublished && snapA.ControlBatchID == blank.ID,
		"snapshot A status=%s control=%d, want published/%d", snapA.Status, snapA.ControlBatchID, blank.ID); err != nil {
		return err
	}

	// 5. 关闭重开：验证持久化恢复。
	if err := st.Close(); err != nil {
		return err
	}
	st2, err := store.OpenStore(path)
	if err != nil {
		return fmt.Errorf("reopen store: %w", err)
	}
	defer st2.Close()
	svc2 := service.New(st2)

	libA2, err := svc2.Store.GetLibrary(libA.ID)
	if err != nil {
		return fmt.Errorf("recover libA: %w", err)
	}
	if err := smokeCheck(libA2.Name == libA.Name && libA2.Status == libA.Status,
		"libA recovered: name=%q status=%q", libA2.Name, libA2.Status); err != nil {
		return err
	}
	profA2, err := svc2.Store.GetDamageProfile(libA.ID)
	if err != nil {
		return fmt.Errorf("recover profile A: %w", err)
	}
	if err := smokeCheck(profA2.NFrags == len(aFrags), "recovered profile n_frags = %d, want %d", profA2.NFrags, len(aFrags)); err != nil {
		return err
	}
	attrsA, err := svc2.Store.ListAttributionsByLibrary(libA.ID)
	if err != nil {
		return err
	}
	if err := smokeCheck(len(attrsA) == 1 && attrsA[0].Status == model.AttribConfirmed,
		"recovered attribution: count=%d status=%s, want 1/confirmed", len(attrsA), attrsA[0].Status); err != nil {
		return err
	}
	snapsA, err := svc2.Store.ListSnapshotsByLibrary(libA.ID)
	if err != nil {
		return err
	}
	if err := smokeCheck(len(snapsA) == 1 && snapsA[0].Status == model.SnapPublished,
		"recovered snapshot: count=%d status=%s, want 1/published", len(snapsA), snapsA[0].Status); err != nil {
		return err
	}
	// 污染批次 B 的排除结果同样恢复。
	lbB2, err := svc2.Store.GetLibrary(libB.ID)
	if err != nil {
		return err
	}
	if err := smokeCheck(lbB2.Status == model.LibNeedsReview,
		"libB recovered status = %s, want needs_review", lbB2.Status); err != nil {
		return err
	}

	_ = os.Remove(path)
	return nil
}
