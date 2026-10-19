package app

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/paranoidi/paras-commander/internal/preview"
)

const ueberzugIdentifier = "pc"

// ueberzugCmdBuffer bounds queued commands; beyond it new ones are dropped so rendering never
// blocks on ueberzugpp.
const ueberzugCmdBuffer = 16

// ueberzugDaemon drives `ueberzugpp layer -so <output>`: one goroutine owns the process and
// writes one JSON command per line to its stdin, respawning it if it exited. Started lazily on
// the first command. The render path only does non-blocking channel sends.
type ueberzugDaemon struct {
	once   sync.Once
	cmds   chan []byte
	stop   chan struct{}
	exited chan struct{}
}

type ueberzugAdd struct {
	Action     string `json:"action"`
	Identifier string `json:"identifier"`
	X          int    `json:"x"`
	Y          int    `json:"y"`
	MaxWidth   int    `json:"max_width"`
	MaxHeight  int    `json:"max_height"`
	Path       string `json:"path"`
}

func ueberzugAddLine(x, y, cols, rows int, path string) []byte {
	b, _ := json.Marshal(ueberzugAdd{"add", ueberzugIdentifier, x, y, cols, rows, path})
	return append(b, '\n')
}

func ueberzugRemoveLine() []byte {
	b, _ := json.Marshal(map[string]string{"action": "remove", "identifier": ueberzugIdentifier})
	return append(b, '\n')
}

func (d *ueberzugDaemon) send(line []byte) {
	d.once.Do(func() {
		d.cmds = make(chan []byte, ueberzugCmdBuffer)
		d.stop = make(chan struct{})
		d.exited = make(chan struct{})
		go d.run(preview.UeberzugOutput(os.Getenv))
	})
	select {
	case d.cmds <- line:
	default:
	}
}

func (d *ueberzugDaemon) show(x, y, cols, rows int, path string) {
	d.send(ueberzugAddLine(x, y, cols, rows, path))
}

func (d *ueberzugDaemon) remove() { d.send(ueberzugRemoveLine()) }

// close stops the process (if ever started) and deletes the PNGs handed to it.
func (d *ueberzugDaemon) close() {
	d.once.Do(func() {}) // nothing started after this
	if d.stop != nil {
		close(d.stop)
		<-d.exited
	}
	preview.RemoveUeberzugFiles()
}

func (d *ueberzugDaemon) run(output string) {
	defer close(d.exited)
	var cmd *exec.Cmd
	var stdin io.WriteCloser
	var done chan struct{}
	defer func() {
		if cmd != nil {
			// EOF on stdin lets ueberzugpp remove its window and socket itself; kill if it lingers.
			_ = stdin.Close()
			select {
			case <-done:
			case <-time.After(500 * time.Millisecond):
				_ = cmd.Process.Kill()
				<-done
			}
		}
	}()
	for {
		select {
		case <-d.stop:
			return
		case line := <-d.cmds:
			if done != nil {
				select {
				case <-done:
					cmd, stdin, done = nil, nil, nil
				default:
				}
			}
			if cmd == nil {
				cmd, stdin, done = startUeberzug(output)
				if cmd == nil {
					continue
				}
			}
			_, _ = stdin.Write(line)
		}
	}
}

func startUeberzug(output string) (*exec.Cmd, io.WriteCloser, chan struct{}) {
	cmd := exec.Command("ueberzugpp", "layer", "-so", output)
	stdin, err := cmd.StdinPipe()
	if err != nil || cmd.Start() != nil {
		return nil, nil, nil
	}
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	return cmd, stdin, done
}
