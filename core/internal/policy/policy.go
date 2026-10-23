// Package policy is the single place that says which copies and renames are
// allowed between machines and nodes.
//
// Rules (from the requirements):
//
//   - FM1 and FM2 of the same machine may copy to each other.
//   - Whole groups (FM1+FM2+BM) may be copied between two machines.
//   - FM and BM never exchange models.
//   - A BM may only be copied to the BM of another machine.
//   - Any node, and a whole group, may be renamed in place.
//   - "Copy and rename" is a copy that targets another name.
package policy

import (
	"errors"
	"strings"

	"afvi/tools/core/internal/i18n"
	"afvi/tools/core/internal/machine"
)

// Ref points at one node of one machine.
type Ref struct {
	Machine string           `json:"machine"`
	Node    machine.NodeKind `json:"node"`
}

func (r Ref) sameMachine(o Ref) bool { return strings.EqualFold(r.Machine, o.Machine) }
func (r Ref) same(o Ref) bool        { return r.sameMachine(o) && r.Node == o.Node }

// CheckCopy decides whether a single node copy src -> dst is allowed.
// rename is true when the copy gets a different model name than the source.
func CheckCopy(tr i18n.T, src, dst Ref, rename bool) error {
	if !src.Node.Valid() || !dst.Node.Valid() {
		return errors.New(tr("p.unknown_node"))
	}
	if src.Node.IsFM() != dst.Node.IsFM() {
		return errors.New(tr("p.fm_bm"))
	}
	if src.same(dst) {
		if src.Node == machine.BM {
			return errors.New(tr("p.bm_self"))
		}
		if !rename {
			return errors.New(tr("p.same_place"))
		}
		return nil
	}
	if src.Node.IsFM() {
		if src.sameMachine(dst) {
			return nil // FM1 <-> FM2 inside one machine
		}
		return errors.New(tr("p.fm_cross"))
	}
	// BM -> BM
	if src.sameMachine(dst) {
		return errors.New(tr("p.bm_other"))
	}
	return nil
}

// CheckGroupCopy decides whether copying the whole group of machine src to
// machine dst is allowed.
func CheckGroupCopy(tr i18n.T, src, dst string) error {
	if strings.EqualFold(src, dst) {
		return errors.New(tr("p.group_same"))
	}
	return nil
}

// Targets lists every node a model of src may be copied to by a single node
// copy. Group copies are listed separately by the caller.
func Targets(src Ref, machines []string) []Ref {
	tr := i18n.For(i18n.En) // only the yes/no answer is used here
	var out []Ref
	for _, m := range machines {
		for _, k := range machine.Kinds {
			d := Ref{Machine: m, Node: k}
			if CheckCopy(tr, src, d, true) == nil {
				out = append(out, d)
			}
		}
	}
	return out
}
