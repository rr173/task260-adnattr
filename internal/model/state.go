package model

import "fmt"

// CanTransition 通用状态机校验：from 必须存在且 to 在其合法后继中。
func CanTransition(table map[string][]string, from, to string) bool {
	allowed, ok := table[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// ValidateLibraryTransition 校验文库批次状态流转。
func ValidateLibraryTransition(from, to string) error {
	if from == to {
		return nil
	}
	if !CanTransition(LibTransitions, from, to) {
		return fmt.Errorf("%w: library %s -> %s", ErrInvalidStatus, from, to)
	}
	return nil
}

// ValidateLibraryMutable rejects writes to a sealed library.
func ValidateLibraryMutable(status string) error {
	if status == LibSealed {
		return ErrSealed
	}
	return nil
}

// ValidateControlLink rejects a direct self-reference between a library and a
// control derived from that same library.
func ValidateControlLink(libraryID, sourceLibraryID int64) error {
	if libraryID > 0 && libraryID == sourceLibraryID {
		return ErrSelfReference
	}
	return nil
}

// ValidateFragmentTransition 校验片段簇状态流转。
func ValidateFragmentTransition(from, to string) error {
	if from == to {
		return nil
	}
	if !CanTransition(FragTransitions, from, to) {
		return fmt.Errorf("%w: fragment %s -> %s", ErrInvalidStatus, from, to)
	}
	return nil
}

// ValidateAttributionTransition 校验归因候选状态流转（open → confirmed）。
func ValidateAttributionTransition(from, to string) error {
	if from == to {
		return nil
	}
	if !CanTransition(AttribStatusTransitions, from, to) {
		return fmt.Errorf("%w: attribution %s -> %s", ErrInvalidStatus, from, to)
	}
	return nil
}

// ValidateSnapshotTransition 校验可信度快照状态流转。
func ValidateSnapshotTransition(from, to string) error {
	if from == to {
		return nil
	}
	if !CanTransition(SnapTransitions, from, to) {
		return fmt.Errorf("%w: snapshot %s -> %s", ErrInvalidStatus, from, to)
	}
	return nil
}

// IsTerminal 判断状态是否为终态（无后继）。
func IsTerminal(table map[string][]string, s string) bool {
	return len(table[s]) == 0
}
