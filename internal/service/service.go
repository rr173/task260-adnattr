// Package service 编排领域包与持久化层，向 HTTP 层暴露高层业务操作。
package service

import (
	"fmt"

	"task260-adnattr/internal/attribution"
	"task260-adnattr/internal/fragment"
	"task260-adnattr/internal/model"
	"task260-adnattr/internal/store"
)

// Service 聚合所有业务操作。
type Service struct {
	Store *store.Store
}

// FragmentInput is the API-independent form used by atomic batch ingestion.
type FragmentInput struct {
	FragLen       int
	C2T5p         float64
	G2A3p         float64
	MeanBaseError float64
	Sequence      string
}

// New 构造 Service。
func New(s *store.Store) *Service { return &Service{Store: s} }

// CreateLibrary 创建文库批次。
func (svc *Service) CreateLibrary(name, note string) (*model.LibraryBatch, error) {
	return svc.Store.CreateLibrary(name, note)
}

// IngestFragment 校验并幂等导入片段摘要。
func (svc *Service) IngestFragment(libID int64, fragLen int, c2t5p, g2a3p, meanErr float64, seq string) (*model.FragmentSummary, bool, error) {
	return fragment.IngestFragment(svc.Store, libID, fragLen, c2t5p, g2a3p, meanErr, seq)
}

// IngestFragmentsAtomic validates every item before committing any batch row.
func (svc *Service) IngestFragmentsAtomic(libID int64, inputs []FragmentInput) (int, int, error) {
	items := make([]store.FragmentInsert, 0, len(inputs))
	for _, input := range inputs {
		length := input.FragLen
		if input.Sequence != "" {
			if err := fragment.ValidateSequence(input.Sequence); err != nil {
				return 0, 0, err
			}
			length = len(input.Sequence)
		}
		if length <= 0 {
			return 0, 0, model.ErrNonPositive
		}
		items = append(items, store.FragmentInsert{
			Fingerprint:   fragment.Fingerprint(libID, length, input.C2T5p, input.G2A3p, input.MeanBaseError),
			FragLen:       length,
			C2T5p:         input.C2T5p,
			G2A3p:         input.G2A3p,
			MeanBaseError: input.MeanBaseError,
		})
	}
	return svc.Store.InsertFragmentBatch(libID, items)
}

// Cluster 将文库片段聚合为片段簇。
func (svc *Service) Cluster(libID int64) ([]*model.FragmentCluster, error) {
	return fragment.ClusterFragments(svc.Store, libID)
}

// ClassifyCluster 更新片段簇状态。
func (svc *Service) ClassifyCluster(id int64, to string) (*model.FragmentCluster, error) {
	c, err := svc.Store.GetCluster(id)
	if err != nil {
		return nil, err
	}
	sealed, err := svc.Store.IsLibrarySealed(c.LibraryID)
	if err != nil {
		return nil, err
	}
	if sealed {
		return nil, model.ErrSealed
	}
	return svc.Store.UpdateClusterStatus(id, to)
}

// AdvanceLibrary 推进文库批次状态机。
func (svc *Service) AdvanceLibrary(id int64, to string) (*model.LibraryBatch, error) {
	return svc.Store.AdvanceLibrary(id, to)
}

// SealLibrary 封存文库批次。
func (svc *Service) SealLibrary(id int64) (*model.LibraryBatch, error) {
	return svc.Store.SealLibrary(id)
}

// CreateControl 创建对照。
func (svc *Service) CreateControl(name string, isBlank bool, meanLen, meanC2T5p, meanG2A3p float64, libID int64) (*model.ControlSample, error) {
	return svc.Store.CreateControl(name, isBlank, meanLen, meanC2T5p, meanG2A3p, libID)
}

// AssociateControl 关联参考对照到文库。
func (svc *Service) AssociateControl(libID, controlID int64) error {
	control, err := svc.Store.GetControl(controlID)
	if err != nil {
		return err
	}
	if err := model.ValidateControlLink(libID, control.LibraryID); err != nil {
		return err
	}
	return svc.Store.AssociateControl(libID, controlID)
}

// ConfirmAttribution 确认归因候选。
func (svc *Service) ConfirmAttribution(id int64) (*model.AttributionCandidate, error) {
	return svc.Store.ConfirmAttribution(id, model.AttribConfirmed)
}

// SelfCheck 健康自检：验证数据库可达且评分逻辑行为正确。
func (svc *Service) SelfCheck() (map[string]interface{}, error) {
	var one int
	if err := svc.Store.DB().QueryRow("SELECT 1").Scan(&one); err != nil {
		return nil, fmt.Errorf("service: self-check db ping: %w", err)
	}
	// 端到端校验评分逻辑不 panic 且对典型古 DNA 轮廓给出 degradation。
	blank := &model.ControlSample{MeanLen: 150, MeanC2T5p: 0.01, MeanG2A3p: 0.01}
	ancient := &model.DamageProfile{Deam5p: 0.2, Deam3p: 0.15, MeanLen: 60, NFrags: 10}
	k, _, _, err := attribution.Score(ancient, blank)
	if err != nil {
		return nil, err
	}
	if k != model.AttribDegradation {
		return nil, fmt.Errorf("service: self-check scoring expected degradation, got %s", k)
	}
	return map[string]interface{}{"db": "ok", "scoring": k}, nil
}
