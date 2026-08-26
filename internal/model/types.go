// Package model 定义古 DNA 损伤与污染归因服务的领域实体、状态枚举与业务错误。
//
// 核心业务对象：
//   - LibraryBatch   文库批次（一次建库测序产物）
//   - FragmentSummary 片段摘要（读段聚合后的长度与末端碱基替换统计）
//   - FragmentCluster 片段簇（同源片段的分组，用于批量判定损伤一致性）
//   - ControlSample  空白/参考对照（负对照用于污染相似度比对）
//   - DamageProfile  末端脱氨损伤轮廓（5' C→T / 3' G→A 与长度分布）
//   - AttributionCandidate 归因候选（降解 / 现代污染 / 证据不足 / 确认）
//   - ConfidenceSnapshot    可信度快照（冻结对照批次的不可变发布物）
package model

import "errors"

// LibraryBatch 状态机：receiving → pending_analysis → needs_review → published → sealed
const (
	LibReceiving       = "receiving"
	LibPendingAnalysis = "pending_analysis"
	LibNeedsReview     = "needs_review"
	LibPublished       = "published"
	LibSealed          = "sealed"
)

// LibTransitions 定义文库批次合法流转。
var LibTransitions = map[string][]string{
	LibReceiving:       {LibPendingAnalysis},
	LibPendingAnalysis: {LibNeedsReview, LibPendingAnalysis},
	LibNeedsReview:     {LibPublished, LibNeedsReview},
	LibPublished:       {LibSealed},
	LibSealed:          {},
}

// FragmentCluster 状态机：raw → damage_consistent | contamination_suspected | low_quality → excluded
const (
	FragRaw                    = "raw"
	FragDamageConsistent       = "damage_consistent"
	FragContaminationSuspected = "contamination_suspected"
	FragLowQuality             = "low_quality"
	FragExcluded               = "excluded"
)

// FragTransitions 定义片段簇合法流转。
var FragTransitions = map[string][]string{
	FragRaw:                    {FragDamageConsistent, FragContaminationSuspected, FragLowQuality, FragExcluded},
	FragDamageConsistent:       {FragExcluded},
	FragContaminationSuspected: {FragExcluded, FragLowQuality},
	FragLowQuality:             {FragExcluded},
	FragExcluded:               {},
}

// AttributionCandidate 类型：归因结论的类别。
const (
	AttribDegradation          = "degradation"
	AttribModernContamination  = "modern_contamination"
	AttribInsufficientEvidence = "insufficient_evidence"
)

// AttributionCandidate 状态机：open → confirmed。
const (
	AttribOpen      = "open"
	AttribConfirmed = "confirmed"
)

// AttribStatusTransitions 定义归因候选合法状态流转。
var AttribStatusTransitions = map[string][]string{
	AttribOpen:      {AttribConfirmed},
	AttribConfirmed: {},
}

// ConfidenceSnapshot 状态机：draft → published → superseded
const (
	SnapDraft      = "draft"
	SnapPublished  = "published"
	SnapSuperseded = "superseded"
)

// SnapTransitions 定义可信度快照合法流转。
var SnapTransitions = map[string][]string{
	SnapDraft:      {SnapPublished},
	SnapPublished:  {SnapSuperseded},
	SnapSuperseded: {},
}

// 领域错误。
var (
	ErrInvalidBase          = errors.New("model: invalid base encoding (only ACGT accepted)")
	ErrSelfReference        = errors.New("model: self reference rejected")
	ErrSealed               = errors.New("model: entity is sealed, mutation rejected")
	ErrUnknownLibrary       = errors.New("model: library batch not found")
	ErrUnknownCluster       = errors.New("model: fragment cluster not found")
	ErrUnknownControl       = errors.New("model: control sample not found")
	ErrControlMissing       = errors.New("model: required control sample missing")
	ErrInvalidStatus        = errors.New("model: invalid status transition")
	ErrDuplicateFingerprint = errors.New("model: duplicate fragment fingerprint (idempotent ignore)")
	ErrBatchCycle           = errors.New("model: control-library cycle rejected")
	ErrEmptyName            = errors.New("model: name must not be empty")
	ErrNonPositive          = errors.New("model: numeric field must be positive")
)

// LibraryBatch 文库批次。
type LibraryBatch struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	CreatedAt int64  `json:"created_at"`
	SealedAt  int64  `json:"sealed_at,omitempty"`
	Note      string `json:"note,omitempty"`
}

// FragmentSummary 片段摘要：片段簇的代表性统计。
type FragmentSummary struct {
	ID            int64   `json:"id"`
	LibraryID     int64   `json:"library_id"`
	Fingerprint   string  `json:"fingerprint"`
	FragLen       int     `json:"frag_len"`
	C2T5p         float64 `json:"c2t_5p"` // 5' 端 C→T 替换率
	G2A3p         float64 `json:"g2a_3p"` // 3' 端 G→A 替换率
	MeanBaseError float64 `json:"mean_base_error"`
	CreatedAt     int64   `json:"created_at"`
}

// FragmentCluster 片段簇。
type FragmentCluster struct {
	ID          int64   `json:"id"`
	LibraryID   int64   `json:"library_id"`
	Fingerprint string  `json:"fingerprint"`
	Status      string  `json:"status"`
	MeanLen     float64 `json:"mean_len"`
	MeanC2T5p   float64 `json:"mean_c2t_5p"`
	MeanG2A3p   float64 `json:"mean_g2a_3p"`
	Size        int     `json:"size"`
	CreatedAt   int64   `json:"created_at"`
}

// ControlSample 空白/参考对照。
type ControlSample struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	IsBlank   bool    `json:"is_blank"`
	MeanLen   float64 `json:"mean_len"`
	MeanC2T5p float64 `json:"mean_c2t_5p"`
	MeanG2A3p float64 `json:"mean_g2a_3p"`
	LibraryID int64   `json:"library_id,omitempty"` // 若该对照自身也是某文库
	CreatedAt int64   `json:"created_at"`
}

// DamageProfile 末端脱氨损伤轮廓。
type DamageProfile struct {
	ID         int64   `json:"id"`
	LibraryID  int64   `json:"library_id"`
	Deam5p     float64 `json:"deam_5p"`
	Deam3p     float64 `json:"deam_3p"`
	MeanLen    float64 `json:"mean_len"`
	NFrags     int     `json:"n_frags"`
	ComputedAt int64   `json:"computed_at"`
}

// AttributionCandidate 归因候选。
type AttributionCandidate struct {
	ID        int64   `json:"id"`
	LibraryID int64   `json:"library_id"`
	Kind      string  `json:"kind"` // degradation | modern_contamination | insufficient_evidence
	Status    string  `json:"status"`
	Score     float64 `json:"score"` // 污染/降解置信分 [0,1]
	Reason    string  `json:"reason"`
	CreatedAt int64   `json:"created_at"`
}

// ConfidenceSnapshot 可信度快照。
type ConfidenceSnapshot struct {
	ID             int64  `json:"id"`
	LibraryID      int64  `json:"library_id"`
	Status         string `json:"status"`
	ControlBatchID int64  `json:"control_batch_id,omitempty"` // 冻结的对照批次
	Payload        string `json:"payload"`                    // JSON 摘要
	CreatedAt      int64  `json:"created_at"`
	PublishedAt    int64  `json:"published_at,omitempty"`
}

// IsValidBase 校验单碱基编码，仅允许 A/C/G/T（大小写不敏感）。
func IsValidBase(b byte) bool {
	switch b {
	case 'A', 'C', 'G', 'T', 'a', 'c', 'g', 't':
		return true
	}
	return false
}
