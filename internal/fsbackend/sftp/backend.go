package sftp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"strings"

	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	pkgsftp "github.com/pkg/sftp"
)

func init() {
	if err := fsbackend.RegisterDefault(New()); err != nil {
		panic(err)
	}
}

// Backend implements fsbackend for sftp:// locations.
type Backend struct {
	pool *Pool
}

// New returns an SFTP backend using DefaultPool.
func New() *Backend {
	return &Backend{pool: DefaultPool}
}

// Scheme implements fsbackend.Backend.
func (b *Backend) Scheme() pathloc.Scheme {
	return pathloc.SchemeSFTP
}

func (b *Backend) withResolvedRemote(ctx context.Context, loc pathloc.Path) (*pkgsftp.Client, string, func(), error) {
	client, release, err := b.pool.withSFTP(ctx, loc)
	if err != nil {
		return nil, "", nil, err
	}
	remote, err := pathloc.SFTPRemotePath(loc)
	if err != nil {
		return client, "", release, err
	}
	resolved, err := resolveRemotePath(client, remote)
	if err != nil {
		return client, "", release, err
	}
	return client, resolved, release, nil
}

func isTransportError(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, net.ErrClosed) {
		return true
	}
	if errors.Is(err, pkgsftp.ErrSSHFxConnectionLost) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "connection lost") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "use of closed network connection")
}

func (b *Backend) completeCall(loc pathloc.Path, client *pkgsftp.Client, release func(), err error) {
	if isTransportError(err) {
		if hostPart, herr := pathloc.SFTPHostPart(loc); herr == nil {
			b.pool.evictClient(hostPart, client)
		}
	}
	if release != nil {
		release()
	}
}

func (b *Backend) withClient(ctx context.Context, loc pathloc.Path, retryable bool, fn func(*pkgsftp.Client, string) error) error {
	attempts := 1
	if retryable {
		attempts = 2
	}
	var last error
	for i := 0; i < attempts; i++ {
		client, remote, release, err := b.withResolvedRemote(ctx, loc)
		if err != nil {
			last = err
			canRetry := retryable && client != nil && isTransportError(err)
			b.completeCall(loc, client, release, err)
			if canRetry {
				continue
			}
			return last
		}
		last = fn(client, remote)
		canRetry := retryable && isTransportError(last)
		b.completeCall(loc, client, release, last)
		if last == nil || !canRetry {
			return last
		}
	}
	return last
}

func (b *Backend) readDir(ctx context.Context, loc pathloc.Path, client *pkgsftp.Client, remoteDir string) ([]os.FileInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	type result struct {
		infos []os.FileInfo
		err   error
	}
	done := make(chan result, 1)
	go func() {
		infos, err := client.ReadDirContext(ctx, remoteDir)
		done <- result{infos: infos, err: err}
	}()
	select {
	case res := <-done:
		return res.infos, res.err
	case <-ctx.Done():
		select {
		case res := <-done:
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			return res.infos, res.err
		default:
			if hostPart, err := pathloc.SFTPHostPart(loc); err == nil {
				b.pool.evictClient(hostPart, client)
			}
			res := <-done
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			return res.infos, res.err
		}
	}
}

// List implements fsbackend.Backend.
func (b *Backend) List(ctx context.Context, dir pathloc.Path) ([]fsbackend.Entry, error) {
	var out []fsbackend.Entry
	err := b.withClient(ctx, dir, true, func(client *pkgsftp.Client, remoteDir string) error {
		infos, err := b.readDir(ctx, dir, client, remoteDir)
		if err != nil {
			return fmt.Errorf("sftp readdir %s: %w", remoteDir, err)
		}
		next := make([]fsbackend.Entry, 0, len(infos))
		for _, info := range infos {
			name := info.Name()
			if name == "." {
				continue
			}
			child, err := dir.Join(name)
			if err != nil {
				return err
			}
			next = append(next, entryFromInfo(child, info))
		}
		out = next
		return nil
	})
	return out, err
}

// Stat implements fsbackend.Backend.
func (b *Backend) Stat(ctx context.Context, loc pathloc.Path) (fsbackend.Entry, error) {
	var entry fsbackend.Entry
	err := b.withClient(ctx, loc, true, func(client *pkgsftp.Client, remote string) error {
		info, err := client.Lstat(remote)
		if err != nil {
			return fmt.Errorf("sftp stat %s: %w", remote, err)
		}
		entry = entryFromInfo(loc, info)
		return nil
	})
	return entry, err
}

// OpenRead implements fsbackend.Backend.
func (b *Backend) OpenRead(ctx context.Context, loc pathloc.Path) (io.ReadCloser, error) {
	var last error
	for attempt := 0; attempt < 2; attempt++ {
		client, remote, release, err := b.withResolvedRemote(ctx, loc)
		if err != nil {
			last = err
			canRetry := client != nil && isTransportError(err)
			b.completeCall(loc, client, release, err)
			if canRetry {
				continue
			}
			return nil, last
		}
		f, err := client.Open(remote)
		if err != nil {
			last = err
			canRetry := isTransportError(err)
			b.completeCall(loc, client, release, err)
			if canRetry && attempt == 0 {
				continue
			}
			return nil, last
		}
		hostPart, err := pathloc.SFTPHostPart(loc)
		if err != nil {
			_ = f.Close()
			b.completeCall(loc, client, release, err)
			return nil, err
		}
		b.pool.convertOpToStream(hostPart)
		return &leasedReadCloser{ReadCloser: f, pool: b.pool, hostPart: hostPart}, nil
	}
	return nil, last
}

// OpenWrite implements fsbackend.Backend.
func (b *Backend) OpenWrite(ctx context.Context, loc pathloc.Path, size int64, opts fsbackend.CreateOpts) (io.WriteCloser, error) {
	_ = size
	client, remote, release, err := b.withResolvedRemote(ctx, loc)
	if err != nil {
		b.completeCall(loc, client, release, err)
		return nil, err
	}
	flags := os.O_WRONLY | os.O_CREATE
	if opts.Truncate {
		flags |= os.O_TRUNC
	}
	if opts.Append {
		flags |= os.O_APPEND
	}
	f, err := client.OpenFile(remote, flags)
	if err != nil {
		b.completeCall(loc, client, release, err)
		return nil, err
	}
	hostPart, err := pathloc.SFTPHostPart(loc)
	if err != nil {
		_ = f.Close()
		b.completeCall(loc, client, release, err)
		return nil, err
	}
	b.pool.convertOpToStream(hostPart)
	return &leasedWriteCloser{WriteCloser: f, pool: b.pool, hostPart: hostPart}, nil
}

// Mkdir implements fsbackend.Backend.
func (b *Backend) Mkdir(ctx context.Context, dir pathloc.Path, perm fs.FileMode) error {
	_ = perm
	return b.withClient(ctx, dir, false, func(client *pkgsftp.Client, remote string) error {
		return client.Mkdir(remote)
	})
}

// Remove implements fsbackend.Backend.
func (b *Backend) Remove(ctx context.Context, loc pathloc.Path) error {
	return b.withClient(ctx, loc, false, func(client *pkgsftp.Client, remote string) error {
		info, err := client.Lstat(remote)
		if err != nil {
			return err
		}
		if info.IsDir() {
			return client.RemoveDirectory(remote)
		}
		return client.Remove(remote)
	})
}

// Rename implements fsbackend.Backend.
func (b *Backend) Rename(ctx context.Context, oldLoc, newLoc pathloc.Path) error {
	client, release, err := b.pool.withSFTP(ctx, oldLoc)
	if err != nil {
		b.completeCall(oldLoc, client, release, err)
		return err
	}
	oldRemote, err := pathloc.SFTPRemotePath(oldLoc)
	if err != nil {
		b.completeCall(oldLoc, client, release, err)
		return err
	}
	oldRemote, err = resolveRemotePath(client, oldRemote)
	if err != nil {
		b.completeCall(oldLoc, client, release, err)
		return err
	}
	newRemote, err := pathloc.SFTPRemotePath(newLoc)
	if err != nil {
		b.completeCall(oldLoc, client, release, err)
		return err
	}
	newRemote, err = resolveRemotePath(client, newRemote)
	if err != nil {
		b.completeCall(oldLoc, client, release, err)
		return err
	}
	err = client.Rename(oldRemote, newRemote)
	b.completeCall(oldLoc, client, release, err)
	return err
}

// ReadSymlink implements fsbackend.Backend.
func (b *Backend) ReadSymlink(ctx context.Context, loc pathloc.Path) (string, error) {
	var target string
	err := b.withClient(ctx, loc, true, func(client *pkgsftp.Client, remote string) error {
		var rerr error
		target, rerr = client.ReadLink(remote)
		return rerr
	})
	return target, err
}

// Symlink implements fsbackend.Backend.
func (b *Backend) Symlink(ctx context.Context, loc pathloc.Path, target string) error {
	return b.withClient(ctx, loc, false, func(client *pkgsftp.Client, remote string) error {
		return client.Symlink(target, remote)
	})
}

func entryFromInfo(loc pathloc.Path, info os.FileInfo) fsbackend.Entry {
	mode := info.Mode()
	t := fsbackend.EntryFile
	switch {
	case mode&fs.ModeSymlink != 0:
		t = fsbackend.EntrySymlink
	case info.IsDir():
		t = fsbackend.EntryDirectory
	case !mode.IsRegular():
		t = fsbackend.EntryOther
	}
	return fsbackend.Entry{
		Name:       info.Name(),
		Loc:        loc,
		Type:       t,
		Size:       info.Size(),
		Mode:       mode,
		ModifiedAt: info.ModTime(),
	}
}

// TouchConn exposes pool touch for tests.
func TouchConn(ctx context.Context, loc pathloc.Path) error {
	return DefaultPool.Touch(ctx, loc)
}

// CloseAllConnections closes every pooled SFTP session.
func CloseAllConnections() {
	DefaultPool.CloseAll()
}
