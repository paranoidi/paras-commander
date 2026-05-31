package pools

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/paranoidi/paras-commander/internal/config"
)

// File is the decoded work-pool definition file (pools.toml).
type File struct {
	Pools []Def
}

// Def is one [[pools]] table in pools.toml.
type Def struct {
	Name        string
	MaxParallel int
}

type fileRaw struct {
	Pool []poolRaw `toml:"pools"`
}

type poolRaw struct {
	Name        string `toml:"name"`
	MaxParallel int    `toml:"max_parallel"`
}

// LoadFile reads and validates pools.toml from path.
func LoadFile(path string) (*File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Decode(b)
}

// Decode parses pools TOML from bytes.
func Decode(data []byte) (*File, error) {
	var raw fileRaw
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("pools.toml: %w", err)
	}
	out := &File{}
	seen := make(map[string]struct{}, len(raw.Pool))
	for i, p := range raw.Pool {
		name := strings.TrimSpace(p.Name)
		if name == "" {
			return nil, fmt.Errorf("pools.toml: pool %d: name is required", i)
		}
		if _, dup := seen[name]; dup {
			return nil, fmt.Errorf("pools.toml: duplicate pool name %q", name)
		}
		seen[name] = struct{}{}
		maxP := p.MaxParallel
		if maxP < 1 {
			maxP = 1
		}
		if maxP > config.DefaultPoolMaxParallel {
			maxP = config.DefaultPoolMaxParallel
		}
		out.Pools = append(out.Pools, Def{
			Name:        name,
			MaxParallel: maxP,
		})
	}
	return out, nil
}
