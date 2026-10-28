package sftp

import "io"

type leasedReadCloser struct {
	io.ReadSeekCloser
	pool     *Pool
	hostPart string
}

func (l *leasedReadCloser) Close() error {
	err := l.ReadSeekCloser.Close()
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
