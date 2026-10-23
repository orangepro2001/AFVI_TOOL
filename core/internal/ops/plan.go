// Package ops plans and runs Model Copy and Rename.
package ops

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"afvi/tools/core/internal/i18n"
	"afvi/tools/core/internal/machine"
	"afvi/tools/core/internal/model"
	"afvi/tools/core/internal/policy"
	"afvi/tools/core/internal/vfs"
)

// Env resolves machine nodes to their shares.
type Env interface {
	Store(ctx context.Context, ref policy.Ref) (model.Store, error)
}

// Modes of a request.
const (
	ModeSingle = "single"
	ModeGroup  = "group"
)

// CopyRequest asks for a copy. In group mode only the Machine of Source and
// Target is used and every node kind is copied to its counterpart.
type CopyRequest struct {
	Mode    string     `json:"mode"`
	Source  policy.Ref `json:"source"`
	Target  policy.Ref `json:"target"`
	Model   string     `json:"model"`
	NewName string     `json:"newName"` // empty keeps the name
}

// RenameRequest asks for a rename in place. In group mode only the Machine
// of Ref is used and the model is renamed on all three nodes.
type RenameRequest struct {
	Mode    string     `json:"mode"`
	Ref     policy.Ref `json:"ref"`
	Model   string     `json:"model"`
	NewName string     `json:"newName"`
}

// StepView is one line of the plan shown to the user.
type StepView struct {
	Action string `json:"action"` // delete | copy | history | rename | skip
	Text   string `json:"text"`
	Danger bool   `json:"danger,omitempty"`
}

// UnitView is the part of a plan that happens on one target node.
type UnitView struct {
	Source  *policy.Ref `json:"source,omitempty"`
	Target  policy.Ref  `json:"target"`
	Model   string      `json:"model"`
	NewName string      `json:"newName"`
	Steps   []StepView  `json:"steps"`
	Files   int         `json:"files"`
	Bytes   int64       `json:"bytes"`
}

// Plan is what the user confirms and what Run executes.
type Plan struct {
	ID       string     `json:"id"`
	Kind     string     `json:"kind"` // copy | rename
	Title    string     `json:"title"`
	Units    []UnitView `json:"units"`
	Warnings []string   `json:"warnings"`
	// Blockers make the plan not executable (read-only target, conflicts).
	Blockers []string `json:"blockers"`
	Files    int      `json:"files"`
	Bytes    int64    `json:"bytes"`

	lang  i18n.Lang
	units []unit
}

type unit interface {
	run(ctx context.Context, p *Progress) error
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func refLabel(r policy.Ref) string { return r.Machine + " " + string(r.Node) }

func describe(b model.Bundle) string {
	var sb strings.Builder
	for i, p := range b.Parts {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(p.Path())
	}
	return sb.String()
}

func humanBytes(n int64) string {
	const u = 1024
	if n < u {
		return fmt.Sprintf("%d B", n)
	}
	f, i := float64(n), 0
	for f >= u && i < 4 {
		f /= u
		i++
	}
	return fmt.Sprintf("%.1f %s", f, []string{"B", "KB", "MB", "GB", "TB"}[i])
}

// PlanCopy validates a copy request and builds the plan. It reads the
// sources fully (a walk) but changes nothing.
func PlanCopy(ctx context.Context, env Env, req CopyRequest) (*Plan, error) {
	tr := i18n.FromContext(ctx)
	if strings.TrimSpace(req.Model) == "" {
		return nil, errors.New(tr("o.no_model"))
	}
	var pairs [][2]policy.Ref
	title := ""
	switch req.Mode {
	case ModeGroup:
		if err := policy.CheckGroupCopy(tr, req.Source.Machine, req.Target.Machine); err != nil {
			return nil, err
		}
		for _, k := range machine.Kinds {
			pairs = append(pairs, [2]policy.Ref{
				{Machine: req.Source.Machine, Node: k}, {Machine: req.Target.Machine, Node: k}})
		}
		title = tr("o.title_copy_group", req.Model, req.Source.Machine, req.Target.Machine)
	case ModeSingle, "":
		pairs = [][2]policy.Ref{{req.Source, req.Target}}
		title = tr("o.title_copy", req.Model, refLabel(req.Source), refLabel(req.Target))
	default:
		return nil, errors.New(tr("o.unknown_mode", req.Mode))
	}
	if req.NewName != "" {
		title += tr("o.title_newname", req.NewName)
	}

	pl := &Plan{ID: newID(), Kind: "copy", Title: title, lang: i18n.From(ctx), Warnings: []string{}, Blockers: []string{}}
	for _, pr := range pairs {
		u, v, err := planCopyUnit(ctx, env, pr[0], pr[1], req, pl)
		if err != nil {
			if req.Mode == ModeGroup && errors.Is(err, errNotOnSource) {
				pl.Warnings = append(pl.Warnings, tr("o.skip_node", refLabel(pr[0]), req.Model))
				continue
			}
			return nil, err
		}
		pl.units = append(pl.units, u)
		pl.Units = append(pl.Units, v)
		pl.Files += v.Files
		pl.Bytes += v.Bytes
	}
	if len(pl.units) == 0 {
		return nil, errors.New(tr("o.nothing_copy"))
	}
	return pl, nil
}

var errNotOnSource = errors.New("model not on source node")

func planCopyUnit(ctx context.Context, env Env, src, dst policy.Ref, req CopyRequest, pl *Plan) (*copyUnit, UnitView, error) {
	tr := i18n.FromContext(ctx)
	ss, err := env.Store(ctx, src)
	if err != nil {
		return nil, UnitView{}, fmt.Errorf("%s: %w", refLabel(src), err)
	}
	ds, err := env.Store(ctx, dst)
	if err != nil {
		return nil, UnitView{}, fmt.Errorf("%s: %w", refLabel(dst), err)
	}
	sb, found, err := model.Locate(ss, req.Model)
	if err != nil {
		return nil, UnitView{}, fmt.Errorf("%s: %w", refLabel(src), err)
	}
	if !found {
		if req.Mode == ModeGroup {
			return nil, UnitView{}, errNotOnSource
		}
		return nil, UnitView{}, errors.New(tr("o.not_on_source", refLabel(src), req.Model))
	}
	newName := req.NewName
	if newName == "" {
		newName = sb.Name
	}
	if err := model.ValidateName(tr, newName); err != nil {
		return nil, UnitView{}, err
	}
	// Group copies were validated as a whole (CheckGroupCopy, same kind to
	// same kind); the single node rules only apply to single node copies.
	if req.Mode != ModeGroup {
		if err := policy.CheckCopy(tr, src, dst, !model.Equal(newName, sb.Name)); err != nil {
			return nil, UnitView{}, err
		}
	}

	db, dfound, err := model.Locate(ds, newName)
	if err != nil {
		return nil, UnitView{}, fmt.Errorf("%s: %w", refLabel(dst), err)
	}

	u := &copyUnit{src: src, dst: dst, ss: ss, ds: ds, srcB: sb, newName: newName, tr: tr}
	if dfound {
		u.existing = db.Parts
	}
	// never delete what we are about to read
	for _, sp := range sb.Parts {
		for _, ep := range u.existing {
			if sp.Share == ep.Share && ss.FS(sp.Share).Describe() == ds.FS(ep.Share).Describe() &&
				model.Equal(sp.Path(), ep.Path()) {
				return nil, UnitView{}, errors.New(tr("o.same_loc", sp.Path()))
			}
		}
	}

	view := UnitView{Source: &src, Target: dst, Model: sb.Name, NewName: newName}
	if ds.ReadOnly() {
		pl.Blockers = append(pl.Blockers, tr("o.ro_target", refLabel(dst)))
	}
	if !sb.Has(model.KindRepo) {
		pl.Warnings = append(pl.Warnings, tr("o.no_repo", refLabel(src)))
	}
	if dfound {
		pl.Warnings = append(pl.Warnings, tr("o.target_exists", refLabel(dst), db.Name))
	}
	for _, ep := range u.existing {
		if !ep.Belongs(newName) {
			return nil, UnitView{}, errors.New(tr("o.internal_belong", ep.Path(), newName))
		}
		view.Steps = append(view.Steps, StepView{Action: "delete", Danger: true,
			Text: tr("o.step_delete", refLabel(dst), ep.Share, strings.ReplaceAll(ep.Path(), "/", `\`))})
	}
	for _, sp := range sb.Parts {
		fsrc := ss.FS(sp.Share)
		items, err := walk(ctx, fsrc, sp.Path())
		if err != nil {
			return nil, UnitView{}, errors.New(tr("o.read_src", sp.Path(), err))
		}
		u.parts = append(u.parts, srcPart{part: sp, items: items})
		files, bytes := summarize(items)
		view.Files += files
		view.Bytes += bytes
		txt := tr("o.step_copy",
			sp.Share, strings.ReplaceAll(sp.Path(), "/", `\`), sp.Share,
			strings.ReplaceAll(vfs.Join(sp.Dir, sp.TargetName(newName)), "/", `\`), files, humanBytes(bytes))
		act := "copy"
		if sp.Kind == model.KindHistory && newName != sb.Name {
			act = "history"
			txt += tr("o.step_his", newName)
		}
		view.Steps = append(view.Steps, StepView{Action: act, Text: txt})
	}
	return u, view, nil
}

// PlanRename validates a rename request and builds the plan.
func PlanRename(ctx context.Context, env Env, req RenameRequest) (*Plan, error) {
	tr := i18n.FromContext(ctx)
	if strings.TrimSpace(req.Model) == "" {
		return nil, errors.New(tr("o.no_model"))
	}
	if err := model.ValidateName(tr, req.NewName); err != nil {
		return nil, err
	}
	var refs []policy.Ref
	title := ""
	switch req.Mode {
	case ModeGroup:
		for _, k := range machine.Kinds {
			refs = append(refs, policy.Ref{Machine: req.Ref.Machine, Node: k})
		}
		title = tr("o.title_rename_group", req.Model, req.NewName, req.Ref.Machine)
	case ModeSingle, "":
		if !req.Ref.Node.Valid() {
			return nil, errors.New(tr("p.unknown_node"))
		}
		refs = []policy.Ref{req.Ref}
		title = tr("o.title_rename", req.Model, req.NewName, refLabel(req.Ref))
	default:
		return nil, errors.New(tr("o.unknown_mode", req.Mode))
	}

	pl := &Plan{ID: newID(), Kind: "rename", Title: title, lang: i18n.From(ctx), Warnings: []string{}, Blockers: []string{}}
	for _, ref := range refs {
		st, err := env.Store(ctx, ref)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", refLabel(ref), err)
		}
		b, found, err := model.Locate(st, req.Model)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", refLabel(ref), err)
		}
		if !found {
			if req.Mode == ModeGroup {
				pl.Warnings = append(pl.Warnings, tr("o.skip_node", refLabel(ref), req.Model))
				continue
			}
			return nil, errors.New(tr("o.not_on_source", refLabel(ref), req.Model))
		}
		if b.Name == req.NewName {
			return nil, errors.New(tr("o.same_name"))
		}
		u := &renameUnit{ref: ref, st: st, bundle: b, newName: req.NewName, tr: tr}
		if !model.Equal(b.Name, req.NewName) {
			other, ofound, err := model.Locate(st, req.NewName)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", refLabel(ref), err)
			}
			if ofound {
				return nil, errors.New(tr("o.already_exists", refLabel(ref), other.Name, describe(other)))
			}
		}
		if st.ReadOnly() {
			pl.Blockers = append(pl.Blockers, tr("o.ro_rename", refLabel(ref)))
		}
		view := UnitView{Target: ref, Model: b.Name, NewName: req.NewName}
		for _, p := range b.Parts {
			view.Steps = append(view.Steps, StepView{Action: "rename",
				Text: tr("o.step_rename", p.Share, strings.ReplaceAll(p.Path(), "/", `\`), p.TargetName(req.NewName))})
		}
		pl.units = append(pl.units, u)
		pl.Units = append(pl.Units, view)
	}
	if len(pl.units) == 0 {
		return nil, errors.New(tr("o.nothing_rename"))
	}
	return pl, nil
}
