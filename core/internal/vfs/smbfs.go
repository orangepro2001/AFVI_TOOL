package vfs

import (
	"fmt"
	"io"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hirochachacha/go-smb2"
)

// SMBFS reads a share over SMB2/3 without mounting it. It exists so the
// Linux dev machine can browse and copy *from* the real machines; it is
// read-only by construction, there is no write path to the network here.
type SMBFS struct {
	host, share, sub string
	user, password   string

	mu   sync.Mutex
	conn net.Conn
	sess *smb2.Session
	sh   *smb2.Share
}

// NewSMB returns a read-only FS for \\host\share[\sub]. The connection is
// made lazily on first use.
func NewSMB(host, share, sub, user, password string) *SMBFS {
	return &SMBFS{host: host, share: share, sub: Clean(sub), user: user, password: password}
}

func (f *SMBFS) Describe() string {
	s := `\\` + f.host + `\` + f.share
	if f.sub != "" {
		s += `\` + strings.ReplaceAll(f.sub, "/", `\`)
	}
	return s
}

func (f *SMBFS) ReadOnly() bool { return true }

func (f *SMBFS) connect() (*smb2.Share, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sh != nil {
		return f.sh, nil
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(f.host, "445"), 5*time.Second)
	if err != nil {
		return nil, err
	}
	d := &smb2.Dialer{Initiator: &smb2.NTLMInitiator{User: f.user, Password: f.password}}
	sess, err := d.Dial(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("smb login to %s as %q: %w", f.host, f.user, err)
	}
	sh, err := sess.Mount(f.share)
	if err != nil {
		sess.Logoff()
		conn.Close()
		return nil, fmt.Errorf("smb mount %s: %w", f.Describe(), err)
	}
	f.conn, f.sess, f.sh = conn, sess, sh
	return sh, nil
}

// Close drops the connection; the next call reconnects.
func (f *SMBFS) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sh == nil {
		return nil
	}
	f.sh.Umount()
	f.sess.Logoff()
	err := f.conn.Close()
	f.sh, f.sess, f.conn = nil, nil, nil
	return err
}

func (f *SMBFS) native(p string) string {
	c := Clean(Join(f.sub, p))
	if c == "" {
		return "."
	}
	return strings.ReplaceAll(c, "/", `\`)
}

// do runs fn, and on a connection level failure reconnects once.
func (f *SMBFS) do(fn func(sh *smb2.Share) error) error {
	sh, err := f.connect()
	if err != nil {
		return err
	}
	err = fn(sh)
	if err != nil && isConnErr(err) {
		f.Close()
		if sh, err2 := f.connect(); err2 == nil {
			return fn(sh)
		}
	}
	return err
}

func isConnErr(err error) bool {
	if _, ok := err.(*smb2.TransportError); ok {
		return true
	}
	if _, ok := err.(*net.OpError); ok {
		return true
	}
	return err == io.EOF || err == io.ErrUnexpectedEOF
}

func (f *SMBFS) ReadDir(p string) ([]Entry, error) {
	var out []Entry
	err := f.do(func(sh *smb2.Share) error {
		fis, err := sh.ReadDir(f.native(p))
		if err != nil {
			return err
		}
		out = out[:0]
		for _, fi := range fis {
			out = append(out, entryOf(fi))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (f *SMBFS) Stat(p string) (Entry, error) {
	var e Entry
	err := f.do(func(sh *smb2.Share) error {
		fi, err := sh.Stat(f.native(p))
		if err != nil {
			return err
		}
		e = entryOf(fi)
		return nil
	})
	return e, err
}

func (f *SMBFS) Open(p string) (io.ReadCloser, error) {
	var rc io.ReadCloser
	err := f.do(func(sh *smb2.Share) error {
		fh, err := sh.Open(f.native(p))
		if err != nil {
			return err
		}
		rc = fh
		return nil
	})
	return rc, err
}

func (f *SMBFS) MkdirAll(string) error                 { return ErrReadOnly }
func (f *SMBFS) Create(string) (io.WriteCloser, error) { return nil, ErrReadOnly }
func (f *SMBFS) Remove(string) error                   { return ErrReadOnly }
func (f *SMBFS) RemoveAll(string) error                { return ErrReadOnly }
func (f *SMBFS) Rename(string, string) error           { return ErrReadOnly }
func (f *SMBFS) Chtimes(string, time.Time) error       { return ErrReadOnly }
