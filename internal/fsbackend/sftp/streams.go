package sftp

import (
	"errors"
	"io"
)

type leasedReadCloser struct {
	io.ReadCloser
	pool     *Pool
	hostPart string
}

// Seek forwards to the remote file so callers reading from an offset (find
// duplicates hashes in chunks) skip ahead instead of re-downloading the prefix.
func (l *leasedReadCloser) Seek(offset int64, whence int) (int64, error) {
	sk, ok := l.ReadCloser.(io.Seeker)
	if !ok {
		return 0, errors.ErrUnsupported
	}
	return sk.Seek(offset, whence)
}

func (l *leasedReadCloser) Close() error {
	err := l.ReadCloser.Close()
	l.pool.releaseStream(l.hostPart)
	return err
}

type leasedWriteCloser struct {
	io.WriteCloser
	pool     *Pool
	hostPart string
}

func (l *leasedWriteCloser) Close() error {
	err := l.WriteCloser.Close()
	l.pool.releaseStream(l.hostPart)
	return err
}
