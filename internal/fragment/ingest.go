// Package fragment 负责片段摘要的校验、指纹计算与幂等导入，以及按统计近似分桶聚合为片段簇。
package fragment

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
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

// canonicalFloatBits 返回 float64 的规范化 64 位整数表示，用于指纹计算。
// 必须保留输入统计值的全部有效精度：通过 math.Float64bits 取其确切位模式，
// 并把 +0.0 与 -0.0 归一为同一表示，NaN 归一为统一哨兵。
// 这样只有统计值真正相同的片段摘要才会产生相同指纹，便于幂等合并；
// 仅末位小数不同的统计值不会被四舍五入到同一指纹而错误地相互吞并。
func canonicalFloatBits(v float64) uint64 {
	if math.IsNaN(v) {
		return 0x7ff8000000000000 // 统一 NaN 哨兵，不参与区分具体 NaN 位模式。
	}
	bits := math.Float64bits(v)
	if bits == 0x8000000000000000 { // -0.0 → 与 +0.0 同等
		bits = 0
	}
	return bits
}

// Fingerprint 由可测内容（文库、长度、末端替换率、平均错误率）计算确定性指纹，
// 用于幂等导入：相同内容的片段摘要只写入一次。
// 各统计值以确切位模式入指纹，保留输入的全部有效精度。
func Fingerprint(libID int64, fragLen int, c2t5p, g2a3p, meanErr float64) string {
	var buf [8 * 5]byte
	binary.BigEndian.PutUint64(buf[0:8], uint64(libID))
	binary.BigEndian.PutUint64(buf[8:16], uint64(fragLen))
	binary.BigEndian.PutUint64(buf[16:24], canonicalFloatBits(c2t5p))
	binary.BigEndian.PutUint64(buf[24:32], canonicalFloatBits(g2a3p))
	binary.BigEndian.PutUint64(buf[32:40], canonicalFloatBits(meanErr))
	h := sha256.Sum256(buf[:])
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
