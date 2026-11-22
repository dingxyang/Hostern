package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/pkg/sftp"
)

type fileSession struct {
	ctx        context.Context
	cancel     context.CancelFunc
	socket     string
	local      filePane
	remote     filePane
	client     *sftp.Client
	connectErr error
	busy       bool
	status     string
	form       *fileForm
}

type filePane struct {
	dir      string
	input    string
	items    []os.FileInfo
	selected string
}

func (p *filePane) show(dir string, items []os.FileInfo) {
	p.dir, p.input, p.items, p.selected = dir, dir, items, ""
}

func (f *fileSession) removeSocketDir() {
	if f.socket != "" {
		_ = os.RemoveAll(filepath.Dir(f.socket))
	}
}

func (a *localTerm) launchSFTP(e entry) {
	if e.Kind != host {
		return
	}
	e.StartupCommand = ""
	e, err := e.validated()
	if err != nil {
		a.fileLaunchError(err)
		return
	}
	// OpenSSH needs space for a temporary suffix within the Unix socket path limit.
	dir, err := os.MkdirTemp("/tmp", "hostern-sftp-")
	if err != nil {
		a.fileLaunchError(err)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	f := &fileSession{ctx: ctx, cancel: cancel, socket: filepath.Join(dir, "ssh")}
	a.launchSession(e, f)
}

func (a *localTerm) fileLaunchError(err error) {
	a.terminalErrors = append(a.terminalErrors, a.tr("无法打开 SFTP：", "Could not open SFTP: ")+err.Error())
}

func (a *localTerm) connectFiles(s *session, host string) {
	f, term := s.files, s.term
	a.runTask(func() func() {
		home, err := os.UserHomeDir()
		var items []os.FileInfo
		if err == nil {
			home, items, err = (fileLocation{}).read(home)
		}
		return func() {
			if f.ctx.Err() != nil {
				return
			}
			f.local.show(home, items)
			if err != nil {
				f.status = err.Error()
			}
		}
	})
	// The PTY starts SSH when first rendered. Wait independently of the UI thread.
	go func() {
		client, err := dialFiles(f.ctx, f.socket, host, term.Done())
		var dir string
		var items []os.FileInfo
		if err == nil {
			dir, items, err = (fileLocation{client: client}).read(".")
		}
		if err != nil && client != nil {
			_ = client.Close()
			client = nil
		}
		if f.ctx.Err() != nil {
			return
		}
		publish := func() {
			if f.ctx.Err() != nil {
				return
			}
			f.client, f.connectErr = client, err
			f.remote.show(dir, items)
			if client != nil && a.active == s {
				a.focusTab = true
			}
		}
		if a.update == nil {
			publish()
			return
		}
		a.update(publish)
		if client == nil {
			return
		}
		_ = client.Wait()
		a.update(func() {
			if f.ctx.Err() == nil {
				f.client = nil
				f.connectErr = errors.New(a.tr("SFTP 连接已断开", "SFTP connection closed"))
			}
		})
	}()
}

func dialFiles(ctx context.Context, socket, host string, masterDone <-chan struct{}) (*sftp.Client, error) {
	ticker := time.NewTicker(75 * time.Millisecond)
	defer ticker.Stop()
	for {
		if info, err := os.Stat(socket); err == nil && info.Mode()&os.ModeSocket != 0 {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-masterDone:
			return nil, errors.New("SSH session exited")
		case <-ticker.C:
		}
	}

	// A failed multiplex request must fail locally instead of authenticating again.
	cmd := exec.CommandContext(ctx, "/usr/bin/ssh", "-F", "/dev/null", "-T", "-S", socket,
		"-o", "ControlMaster=no", "-o", "ProxyCommand=/usr/bin/false", "-s", "--", host, "sftp")
	var diagnostics bytes.Buffer
	cmd.Stderr = &diagnostics
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = in.Close()
		}
	}()
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = out.Close()
		}
	}()
	if err = cmd.Start(); err != nil {
		return nil, err
	}
	client, err := sftp.NewClientPipe(out, in)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		if message := strings.TrimSpace(diagnostics.String()); message != "" {
			err = fmt.Errorf("%w: %s", err, message)
		}
		return nil, err
	}
	go func() {
		_ = client.Wait()
		_ = cmd.Wait()
	}()
	return client, nil
}

type fileLocation struct {
	client *sftp.Client
	dir    string
}

func (l fileLocation) resolve(name string) string {
	if l.client != nil {
		if path.IsAbs(name) {
			return path.Clean(name)
		}
		return path.Join(l.dir, name)
	}
	if filepath.IsAbs(name) {
		return filepath.Clean(name)
	}
	return filepath.Join(l.dir, name)
}

func (l fileLocation) read(dir string) (string, []os.FileInfo, error) {
	dir = l.resolve(dir)
	var items []os.FileInfo
	var err error
	if l.client != nil {
		dir, err = l.client.RealPath(dir)
		if err == nil {
			items, err = l.client.ReadDir(dir)
			items = slices.DeleteFunc(items, func(item os.FileInfo) bool {
				return !validFileName(item.Name())
			})
		}
	} else {
		dir, err = filepath.EvalSymlinks(dir)
		if err == nil {
			var entries []os.DirEntry
			entries, err = os.ReadDir(dir)
			for _, entry := range entries {
				if info, infoErr := entry.Info(); infoErr == nil {
					items = append(items, info)
				}
			}
		}
	}
	slices.SortFunc(items, func(a, b os.FileInfo) int {
		if a.IsDir() != b.IsDir() {
			if a.IsDir() {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Name(), b.Name())
	})
	return dir, items, err
}

func validFileName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsRune(name, '/') &&
		!strings.ContainsFunc(name, unicode.IsControl)
}

func (l fileLocation) mkdir(name string) error {
	target := l.resolve(name)
	if l.client != nil {
		return l.client.Mkdir(target)
	}
	return os.Mkdir(target, 0o755)
}

func (l fileLocation) rename(old, name string) error {
	from, to := l.resolve(old), l.resolve(name)
	stat := os.Lstat
	if l.client != nil {
		stat = l.client.Lstat
	}
	if _, err := stat(to); !os.IsNotExist(err) {
		if err == nil {
			return &os.PathError{Op: "rename", Path: to, Err: os.ErrExist}
		}
		return err
	}
	if l.client != nil {
		return l.client.Rename(from, to)
	}
	return os.Rename(from, to)
}

func (l fileLocation) remove(name string, directory bool) error {
	target := l.resolve(name)
	if l.client != nil {
		if directory {
			return l.client.RemoveDirectory(target)
		}
		return l.client.Remove(target)
	}
	return os.Remove(target)
}

func (l fileLocation) open(name string) (io.ReadCloser, error) {
	target := l.resolve(name)
	if l.client != nil {
		return l.client.Open(target)
	}
	return os.Open(target)
}

func (l fileLocation) create(name string) (io.WriteCloser, error) {
	target := l.resolve(name)
	const flags = os.O_WRONLY | os.O_CREATE | os.O_EXCL
	if l.client != nil {
		return l.client.OpenFile(target, flags)
	}
	return os.OpenFile(target, flags, 0o644)
}

func copySFTPFile(ctx context.Context, source, destination fileLocation, name string) (err error) {
	if !validFileName(name) {
		return fmt.Errorf("invalid file name %q", name)
	}
	in, err := source.open(name)
	if err != nil {
		return err
	}
	defer in.Close()
	target := name
	// Publish from a staging file when the destination supports hard links.
	stage := destination.client == nil
	if !stage {
		_, stage = destination.client.HasExtension("hardlink@openssh.com")
	}
	if stage {
		target = ".hostern-transfer-" + rand.Text()
	}
	out, err := destination.create(target)
	if err != nil {
		return err
	}
	defer func() {
		if stage || err != nil {
			err = errors.Join(err, destination.remove(target, false))
		}
	}()
	_, copyErr := io.Copy(out, cancelFileRead{ctx: ctx, reader: in})
	if err := errors.Join(copyErr, out.Close()); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !stage {
		return nil
	}
	from, to := destination.resolve(target), destination.resolve(name)
	if destination.client != nil {
		return destination.client.Link(from, to)
	}
	return os.Link(from, to)
}

type cancelFileRead struct {
	ctx    context.Context
	reader io.Reader
}

func (r cancelFileRead) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}
