package sftp

import (
	"context"
	"fmt"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	pkgsftp "github.com/pkg/sftp"
	"io"
	"io/fs"
	"os"
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

func (b *Backend) withResolvedRemote(ctx context.Context, loc pathloc.Path) (*pkgsftp.Client, string, error) {
	client, err := b.pool.withSFTP(ctx, loc)
	if err != nil {
		return nil, "", err
	}
	remote, err := pathloc.SFTPRemotePath(loc)
	if err != nil {
		return nil, "", err
	}
	resolved, err := resolveRemotePath(client, remote)
	if err != nil {
		return nil, "", err
	}
	return client, resolved, nil
}

// List implements fsbackend.Backend.
func (b *Backend) List(ctx context.Context, dir pathloc.Path) ([]fsbackend.Entry, error) {
	client, remoteDir, err := b.withResolvedRemote(ctx, dir)
	if err != nil {
		return nil, err
	}
	infos, err := client.ReadDir(remoteDir)
	if err != nil {
		return nil, fmt.Errorf("sftp readdir %s: %w", remoteDir, err)
	}
	out := make([]fsbackend.Entry, 0, len(infos))
	for _, info := range infos {
		name := info.Name()
		if name == "." {
			continue
		}
		child, err := dir.Join(name)
		if err != nil {
			return nil, err
		}
		out = append(out, entryFromInfo(child, info))
	}
	return out, nil
}

// Stat implements fsbackend.Backend.
func (b *Backend) Stat(ctx context.Context, loc pathloc.Path) (fsbackend.Entry, error) {
	client, remote, err := b.withResolvedRemote(ctx, loc)
	if err != nil {
		return fsbackend.Entry{}, err
	}
	info, err := client.Lstat(remote)
	if err != nil {
		return fsbackend.Entry{}, fmt.Errorf("sftp stat %s: %w", remote, err)
	}
	return entryFromInfo(loc, info), nil
}

// OpenRead implements fsbackend.Backend.
func (b *Backend) OpenRead(ctx context.Context, loc pathloc.Path) (io.ReadCloser, error) {
	client, remote, err := b.withResolvedRemote(ctx, loc)
	if err != nil {
		return nil, err
	}
	f, err := client.Open(remote)
	if err != nil {
		return nil, err
	}
	hostPart, err := pathloc.SFTPHostPart(loc)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	b.pool.leaseStream(hostPart)
	return &leasedReadCloser{ReadCloser: f, pool: b.pool, hostPart: hostPart}, nil
}

// OpenWrite implements fsbackend.Backend.
func (b *Backend) OpenWrite(ctx context.Context, loc pathloc.Path, size int64, opts fsbackend.CreateOpts) (io.WriteCloser, error) {
	_ = size
	client, remote, err := b.withResolvedRemote(ctx, loc)
	if err != nil {
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
		return nil, err
	}
	hostPart, err := pathloc.SFTPHostPart(loc)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	b.pool.leaseStream(hostPart)
	return &leasedWriteCloser{WriteCloser: f, pool: b.pool, hostPart: hostPart}, nil
}

// Mkdir implements fsbackend.Backend.
func (b *Backend) Mkdir(ctx context.Context, dir pathloc.Path, perm fs.FileMode) error {
	client, remote, err := b.withResolvedRemote(ctx, dir)
	if err != nil {
		return err
	}
	return client.Mkdir(remote)
}

// Remove implements fsbackend.Backend.
func (b *Backend) Remove(ctx context.Context, loc pathloc.Path) error {
	client, remote, err := b.withResolvedRemote(ctx, loc)
	if err != nil {
		return err
	}
	info, err := client.Lstat(remote)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return client.RemoveDirectory(remote)
	}
	return client.Remove(remote)
}

// Rename implements fsbackend.Backend.
func (b *Backend) Rename(ctx context.Context, oldLoc, newLoc pathloc.Path) error {
	client, err := b.pool.withSFTP(ctx, oldLoc)
	if err != nil {
		return err
	}
	oldRemote, err := pathloc.SFTPRemotePath(oldLoc)
	if err != nil {
		return err
	}
	oldRemote, err = resolveRemotePath(client, oldRemote)
	if err != nil {
		return err
	}
	newRemote, err := pathloc.SFTPRemotePath(newLoc)
	if err != nil {
		return err
	}
	newRemote, err = resolveRemotePath(client, newRemote)
	if err != nil {
		return err
	}
	return client.Rename(oldRemote, newRemote)
}

// ReadSymlink implements fsbackend.Backend.
func (b *Backend) ReadSymlink(ctx context.Context, loc pathloc.Path) (string, error) {
	client, remote, err := b.withResolvedRemote(ctx, loc)
	if err != nil {
		return "", err
	}
	return client.ReadLink(remote)
}

// Symlink implements fsbackend.Backend.
func (b *Backend) Symlink(ctx context.Context, loc pathloc.Path, target string) error {
	client, remote, err := b.withResolvedRemote(ctx, loc)
	if err != nil {
		return err
	}
	return client.Symlink(target, remote)
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
