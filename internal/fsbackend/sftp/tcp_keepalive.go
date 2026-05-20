package sftp

import (
	"net"
	"time"
)

// defaultTCPKeepalivePeriod is the kernel probe interval on the SSH TCP socket.
// It does not require SSH server extensions—only local + peer TCP stacks (LAN-friendly).
const defaultTCPKeepalivePeriod = 30 * time.Second

func enableTCPKeepAlive(conn net.Conn) {
	tcp, ok := conn.(*net.TCPConn)
	if !ok {
		return
	}
	_ = tcp.SetKeepAlive(true)
	_ = tcp.SetKeepAlivePeriod(defaultTCPKeepalivePeriod)
}
