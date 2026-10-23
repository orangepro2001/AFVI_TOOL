package ops

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"afvi/tools/core/internal/vfs"
)

// item is one file or directory below a part, with a path relative to it.
type item struct {
	Rel   string
	IsDir bool
	Size  int64
	Mod   time.Time
}

// walk lists root and everything below it, directories before their content.
func walk(ctx context.Context, f vfs.FS, root string) ([]item, error) {
	st, err := f.Stat(root)
	if err != nil {
		return nil, err
	}
	out := []item{{Rel: "", IsDir: st.IsDir, Size: st.Size, Mod: st.ModTime}}
	if !st.IsDir {
		return out, nil
	}
	var rec func(rel string) error
	rec = func(rel string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		es, err := f.ReadDir(vfs.Join(root, rel))
		if err != nil {
			return err
		}
		for _, e := range es {
			r := e.Name
			if rel != "" {
				r = rel + "/" + e.Name
			}
			out = append(out, item{Rel: r, IsDir: e.IsDir, Size: e.Size, Mod: e.ModTime})
			if e.IsDir {
				if err := rec(r); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := rec(""); err != nil {
		return nil, err
	}
	return out, nil
}

func summarize(items []item) (files int, bytes int64) {
	for _, it := range items {
		if !it.IsDir {
			files++
			bytes += it.Size
		}
	}
	return
}

// sameTree compares two walks by relative path and file size.
func sameTree(a, b []item) error {
	key := func(items []item) []string {
		k := make([]string, 0, len(items))
		for _, it := range items {
			if it.IsDir {
				k = append(k, it.Rel+"/")
			} else {
				k = append(k, fmt.Sprintf("%s|%d", it.Rel, it.Size))
			}
		}
		sort.Strings(k)
		return k
	}
	ka, kb := key(a), key(b)
	if len(ka) != len(kb) {
		return fmt.Errorf("entry count differs: source %d, target %d", len(ka), len(kb))
	}
	for i := range ka {
		if ka[i] != kb[i] {
			return fmt.Errorf("entry differs: source %q, target %q", ka[i], kb[i])
		}
	}
	return nil
}

// copyFile streams src to dst, counting bytes through onBytes.
func copyFile(ctx context.Context, src vfs.FS, sp string, dst vfs.FS, dp string, size int64, onBytes func(int64)) error {
	r, err := src.Open(sp)
	if err != nil {
		return err
	}
	defer r.Close()
	w, err := dst.Create(dp)
	if err != nil {
		return err
	}
	buf := make([]byte, 1<<20)
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			w.Close()
			return err
		}
		n, rerr := r.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				w.Close()
				return werr
			}
			total += int64(n)
			onBytes(int64(n))
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			w.Close()
			return rerr
		}
	}
	if err := w.Close(); err != nil {
		return err
	}
	if total != size {
		return fmt.Errorf("%s: read %d bytes, expected %d", sp, total, size)
	}
	return nil
}

// copyTree copies the part at sp (file or directory) to dp.
func copyTree(ctx context.Context, src vfs.FS, sp string, dst vfs.FS, dp string, items []item, p *Progress) error {
	for _, it := range items {
		s, d := sp, dp
		if it.Rel != "" {
			s, d = vfs.Join(sp, it.Rel), vfs.Join(dp, it.Rel)
		}
		if it.IsDir {
			if err := dst.MkdirAll(d); err != nil {
				return err
			}
			continue
		}
		p.setCurrent(d)
		if err := copyFile(ctx, src, s, dst, d, it.Size, p.addBytes); err != nil {
			return fmt.Errorf("copying %s: %w", d, err)
		}
		if err := dst.Chtimes(d, it.Mod); err != nil {
			return fmt.Errorf("setting time of %s: %w", d, err)
		}
		p.addFile()
	}
	// directory times last, deepest first, so file writes do not change them
	for i := len(items) - 1; i >= 0; i-- {
		it := items[i]
		if !it.IsDir {
			continue
		}
		d := dp
		if it.Rel != "" {
			d = vfs.Join(dp, it.Rel)
		}
		if err := dst.Chtimes(d, it.Mod); err != nil && !strings.Contains(err.Error(), "not supported") {
			return fmt.Errorf("setting time of %s: %w", d, err)
		}
	}
	return nil
}
