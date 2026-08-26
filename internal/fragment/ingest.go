// Package fragment 负责片段摘要的校验、指纹计算与幂等导入，以及按统计近似分桶聚合为片段簇。
package fragment

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"task260-adnattr/internal/model"
	"task260-adnattr/internal/store"
)

// ValidateSequence 校验原始碱基序列：仅允许 A/C/G/T（大小写不敏感）。
func ValidateSequence(seq string) error {
	s := strings.ToUpper(seq)
	for i := 0; i < len(s); i++ {
		if !model.IsValidBase(s[i]) {
			return fmt.Errorf("%w: %q at pos %d", model.ErrInvalidBase, string(s[i]), i)
		}
	}
	return nil
}

// Fingerprint 由可测内容（文库、长度、末端替换率、平均错误率）计算确定性指纹，
// 用于幂等导入：相同内容的片段摘要只写入一次。
func Fingerprint(libID int64, fragLen int, c2t5p, g2a3p, meanErr float64) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("lib=%d|len=%d|c2t=%.4f|g2a=%.4f|err=%.4f", libID, fragLen, c2t5p, g2a3p, meanErr)))
	return hex.EncodeToString(h[:])
}

// IngestFragment 校验（可选原始序列）并计算指纹后幂等导入片段摘要。
// 若提供 seq，则以 seq 长度作为片段长度并强制碱基校验；否则使用入参 fragLen。
// 返回导入后的实体与 added（true=新写入，false=指纹已存在被忽略）。
func IngestFragment(s *store.Store, libID int64, fragLen int, c2t5p, g2a3p, meanErr float64, seq string) (*model.FragmentSummary, bool, error) {
	if seq != "" {
		if err := ValidateSequence(seq); err != nil {
			return nil, false, err
		}
		fragLen = len(seq)
	}
	fp := Fingerprint(libID, fragLen, c2t5p, g2a3p, meanErr)
	return s.InsertFragmentSummary(libID, fp, fragLen, c2t5p, g2a3p, meanErr)
}
