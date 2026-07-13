package chromastyles

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/styles"
)

// mu guards custom/loadWarnings: multiple App instances (e.g. in tests) can load
// config and render previews concurrently, both touching this process-wide registry.
var mu sync.RWMutex

var (
	custom       = map[string]*chroma.Style{}
	loadWarnings []error
)

// LoadWarnings returns non-fatal issues from the most recent LoadFromDir call.
func LoadWarnings() []error {
	mu.RLock()
	defer mu.RUnlock()
	return loadWarnings
}

// ResetForTest clears custom styles and load warnings between tests.
func ResetForTest() {
	mu.Lock()
	defer mu.Unlock()
	custom = map[string]*chroma.Style{}
	loadWarnings = nil
}

// LoadFromDir loads Chroma XML styles from dir into the custom registry.
// The custom map is replaced on each call. Invalid sibling .xml files are skipped
// and recorded in LoadWarnings; a missing directory is not an error.
func LoadFromDir(dir string) error {
	mu.Lock()
	defer mu.Unlock()
	custom = map[string]*chroma.Style{}
	loadWarnings = nil

	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read preview styles dir %q: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".xml" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if err := loadFile(path); err != nil {
			loadWarnings = append(loadWarnings, err)
		}
	}
	if len(loadWarnings) > 0 {
		return fmt.Errorf("load preview styles from %q: %w", dir, errors.Join(loadWarnings...))
	}
	return nil
}

func loadFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	style, err := chroma.NewXMLStyle(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	name := strings.ToLower(strings.TrimSpace(style.Name))
	if name == "" {
		return fmt.Errorf("%s: style name is required", filepath.Base(path))
	}
	if name == "default" {
		return fmt.Errorf("%s: style name %q is reserved", filepath.Base(path), style.Name)
	}
	if _, exists := custom[name]; exists {
		return fmt.Errorf("%s: duplicate preview style name %q", filepath.Base(path), style.Name)
	}
	custom[name] = style
	return nil
}

// Get returns a custom preview style or a built-in Chroma style.
func Get(name string) *chroma.Style {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		return styles.Fallback
	}
	mu.RLock()
	style, ok := custom[key]
	mu.RUnlock()
	if ok {
		return style
	}
	return styles.Get(name)
}

// IsValid reports whether name resolves to a custom or built-in Chroma style.
func IsValid(name string) bool {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		return false
	}
	mu.RLock()
	_, ok := custom[key]
	mu.RUnlock()
	if ok {
		return true
	}
	_, ok = styles.Registry[key]
	return ok
}

// CanonicalName returns the lowercase registry key for a valid style name.
func CanonicalName(name string) string {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		return ""
	}
	mu.RLock()
	_, ok := custom[key]
	mu.RUnlock()
	if ok {
		return key
	}
	if _, ok := styles.Registry[key]; ok {
		return key
	}
	return ""
}
