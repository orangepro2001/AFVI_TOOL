//go:build linux

package vfs

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

var networkFSTypes = map[string]bool{
	"cifs": true, "smb3": true, "smbfs": true, "nfs": true, "nfs4": true,
	"fuse.sshfs": true, "fuse.gvfsd-fuse": true, "9p": true, "afs": true, "ceph": true,
}

// isNetworkPath reports whether p lies on a network file system, using the
// longest matching mount point in /proc/self/mountinfo.
func isNetworkPath(p string) bool {
	abs, err := filepath.Abs(p)
	if err != nil {
		return false
	}
	if r, err := filepath.EvalSymlinks(abs); err == nil {
		abs = r
	}
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return false
	}
	defer f.Close()
	best, bestNet := "", false
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		// 36 35 98:0 /mnt1 /mnt2 rw,noatime master:1 - ext3 /dev/root rw
		line := sc.Text()
		pre, post, ok := strings.Cut(line, " - ")
		if !ok {
			continue
		}
		pf := strings.Fields(pre)
		qf := strings.Fields(post)
		if len(pf) < 5 || len(qf) < 1 {
			continue
		}
		mp := unescapeMount(pf[4])
		if abs != mp && !strings.HasPrefix(abs, strings.TrimSuffix(mp, "/")+"/") {
			continue
		}
		if len(mp) >= len(best) {
			best, bestNet = mp, networkFSTypes[qf[0]]
		}
	}
	return bestNet
}

func unescapeMount(s string) string {
	r := strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`)
	return r.Replace(s)
}
