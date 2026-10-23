package ops

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"afvi/tools/core/internal/i18n"
	"afvi/tools/core/internal/model"
	"afvi/tools/core/internal/policy"
	"afvi/tools/core/internal/vfs"
)

// ErrBlocked is returned by Run for a plan with blockers.
var ErrBlocked = errors.New("plan is blocked")

// Run executes a plan. Progress is updated while it runs. A plan runs at
// most once; the caller owns that rule.
func Run(ctx context.Context, pl *Plan, p *Progress) error {
	if len(pl.Blockers) > 0 {
		return fmt.Errorf("%w: %s", ErrBlocked, strings.Join(pl.Blockers, "; "))
	}
	if pl.Kind == "rename" {
		return runRename(ctx, pl, p)
	}
	for _, u := range pl.units {
		if err := u.run(ctx, p); err != nil {
			return err
		}
	}
	p.setPhase(i18n.For(pl.lang)("e.done"))
	return nil
}

type srcPart struct {
	part  model.Part
	items []item
}

// copyUnit is the atomic "delete the target, then copy the new one" for one
// node: the user sees it as one operation, a failure leaves no half copy.
type copyUnit struct {
	src, dst policy.Ref
	ss, ds   model.Store
	srcB     model.Bundle
	newName  string
	existing []model.Part
	parts    []srcPart
	tr       i18n.T
}

func (u *copyUnit) label() string { return refLabel(u.dst) }

func (u *copyUnit) run(ctx context.Context, p *Progress) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	// Re-check the sources right before the destructive step: everything
	// must still be readable and of the planned size.
	p.setPhase(u.tr("e.check_src", u.label()))
	for _, sp := range u.parts {
		cur, werr := walk(ctx, u.ss.FS(sp.part.Share), sp.part.Path())
		if werr != nil {
			return errors.New(u.tr("e.src_unreadable", sp.part.Path(), werr))
		}
		if werr := sameTree(sp.items, cur); werr != nil {
			return errors.New(u.tr("e.src_changed", sp.part.Path(), werr))
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	// ---- point of no return -------------------------------------------
	p.commit()
	ctx = context.WithoutCancel(ctx)

	p.setPhase(u.tr("e.delete", u.label()))
	for _, ep := range u.existing {
		if !ep.Belongs(u.newName) {
			return errors.New(u.tr("e.refuse_delete", ep.Path(), u.newName))
		}
		fsys := u.ds.FS(ep.Share)
		if rerr := fsys.RemoveAll(ep.Path()); rerr != nil && !vfs.IsNotExist(rerr) {
			return errors.New(u.tr("e.delete_err", ep.Path(), rerr))
		}
		p.logf(u.tr("e.deleted", ep.Path()))
	}

	var created []model.Part
	defer func() {
		if err == nil {
			return
		}
		// A failed copy must not leave half a model on a production share.
		for _, cp := range created {
			if rerr := u.ds.FS(cp.Share).RemoveAll(cp.Path()); rerr != nil && !vfs.IsNotExist(rerr) {
				p.logf(u.tr("e.cleanup_fail", cp.Path(), rerr))
			} else {
				p.logf(u.tr("e.cleaned", cp.Path()))
			}
		}
		err = errors.New(u.tr("e.copy_failed", u.label(), err))
	}()

	p.setPhase(u.tr("e.copy", u.label()))
	for _, sp := range u.parts {
		dname := sp.part.TargetName(u.newName)
		dpart := model.Part{Kind: sp.part.Kind, Share: sp.part.Share, Dir: sp.part.Dir, Name: dname,
			IsDir: sp.part.IsDir, Suffix: sp.part.Suffix}
		dp := dpart.Path()
		sfs, dfs := u.ss.FS(sp.part.Share), u.ds.FS(sp.part.Share)
		if sp.part.Dir != "" {
			if merr := dfs.MkdirAll(sp.part.Dir); merr != nil {
				return merr
			}
		}
		created = append(created, dpart)
		if sp.part.Kind == model.KindHistory {
			if herr := copyHistory(sfs, sp.part.Path(), dfs, dp, u.srcB.Name, u.newName, sp.items[0], p); herr != nil {
				return herr
			}
			continue
		}
		if cerr := copyTree(ctx, sfs, sp.part.Path(), dfs, dp, sp.items, p); cerr != nil {
			return cerr
		}
	}

	p.setPhase(u.tr("e.verify", u.label()))
	for _, sp := range u.parts {
		if sp.part.Kind == model.KindHistory {
			continue // content was rewritten, size differs by design
		}
		dpart := model.Part{Share: sp.part.Share, Dir: sp.part.Dir, Name: sp.part.TargetName(u.newName)}
		got, werr := walk(ctx, u.ds.FS(sp.part.Share), dpart.Path())
		if werr != nil {
			return errors.New(u.tr("e.verify_err", dpart.Path(), werr))
		}
		if werr := sameTree(sp.items, got); werr != nil {
			return errors.New(u.tr("e.verify_err", dpart.Path(), werr))
		}
	}
	p.logf(u.tr("e.done_unit", u.label()))
	return nil
}

func readAll(f vfs.FS, path string) ([]byte, error) {
	r, err := f.Open(path)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

func writeAll(f vfs.FS, path string, data []byte) error {
	w, err := f.Create(path)
	if err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}

// copyHistory copies a .his file, renaming the model in its first column.
func copyHistory(sfs vfs.FS, sp string, dfs vfs.FS, dp, oldName, newName string, it item, p *Progress) error {
	data, err := readAll(sfs, sp)
	if err != nil {
		return err
	}
	if oldName != newName {
		data, _ = model.RewriteHistory(data, oldName, newName)
	}
	p.setCurrent(dp)
	if err := writeAll(dfs, dp, data); err != nil {
		return fmt.Errorf("writing %s: %w", dp, err)
	}
	if err := dfs.Chtimes(dp, it.Mod); err != nil {
		return err
	}
	p.addBytes(int64(len(data)))
	p.addFile()
	return nil
}

// ---- rename ---------------------------------------------------------------

type renameUnit struct {
	ref     policy.Ref
	st      model.Store
	bundle  model.Bundle
	newName string
	tr      i18n.T
}

// undo is a step that reverts one completed rename.
type undo struct {
	text string
	fn   func() error
}

func (u *renameUnit) apply(p *Progress, journal *[]undo) error {
	p.setPhase(u.tr("e.rename", refLabel(u.ref)))
	// directories first (atomic renames), the history file last
	var his *model.Part
	for i := range u.bundle.Parts {
		part := u.bundle.Parts[i]
		if part.Kind == model.KindHistory {
			his = &u.bundle.Parts[i]
			continue
		}
		fsys := u.st.FS(part.Share)
		from := part.Path()
		to := vfs.Join(part.Dir, part.TargetName(u.newName))
		if err := fsys.Rename(from, to); err != nil {
			return errors.New(u.tr("e.rename_err", from, to, err))
		}
		p.logf(fmt.Sprintf("%s → %s", from, to))
		*journal = append(*journal, undo{text: to + " → " + from, fn: func() error { return fsys.Rename(to, from) }})
	}
	if his == nil {
		return nil
	}
	fsys := u.st.FS(his.Share)
	from := his.Path()
	to := vfs.Join(his.Dir, his.TargetName(u.newName))
	data, err := readAll(fsys, from)
	if err != nil {
		return fmt.Errorf("reading %s: %w", from, err)
	}
	newData, _ := model.RewriteHistory(data, u.bundle.Name, u.newName)
	tmp := to + ".afvi-tmp"
	if err := writeAll(fsys, tmp, newData); err != nil {
		fsys.Remove(tmp)
		return fmt.Errorf("writing %s: %w", tmp, err)
	}
	if err := fsys.Remove(from); err != nil {
		fsys.Remove(tmp)
		return fmt.Errorf("deleting %s: %w", from, err)
	}
	if err := fsys.Rename(tmp, to); err != nil {
		// put the old file back from memory
		if rerr := writeAll(fsys, from, data); rerr != nil {
			return fmt.Errorf("renaming %s failed and the old file could not be restored: %v / %v", from, err, rerr)
		}
		fsys.Remove(tmp)
		return fmt.Errorf("renaming %s → %s: %w", tmp, to, err)
	}
	p.logf(u.tr("e.his_updated", from, to))
	*journal = append(*journal, undo{text: to + " → " + from, fn: func() error {
		if err := fsys.Remove(to); err != nil {
			return err
		}
		return writeAll(fsys, from, data)
	}})
	return nil
}

func runRename(ctx context.Context, pl *Plan, p *Progress) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p.commit()
	var journal []undo
	for _, un := range pl.units {
		u := un.(*renameUnit)
		if err := u.apply(p, &journal); err != nil {
			p.setPhase(u.tr("e.rollback"))
			var failed []string
			for i := len(journal) - 1; i >= 0; i-- {
				if rerr := journal[i].fn(); rerr != nil {
					failed = append(failed, fmt.Sprintf("%s: %v", journal[i].text, rerr))
				} else {
					p.logf(u.tr("e.rolled_part", journal[i].text))
				}
			}
			if len(failed) > 0 {
				return errors.New(u.tr("e.rollback_failed", err, strings.Join(failed, "; ")))
			}
			return errors.New(u.tr("e.rolled_back", err))
		}
		p.addFile()
	}
	p.setPhase(i18n.For(pl.lang)("e.done"))
	return nil
}

// run lets renameUnit satisfy unit; renames are executed by runRename so
// that one journal spans the whole group.
func (u *renameUnit) run(context.Context, *Progress) error {
	return errors.New("renameUnit runs through runRename")
}
