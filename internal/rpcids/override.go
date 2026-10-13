package rpcids

import (
	"encoding/json/v2"
	"fmt"
	"os"
	"slices"
	"strings"
)

// LoadOverrides reads {"logical.name": "wireId", ...} and applies it.
// Unknown names are rejected so typos do not silently do nothing.
func LoadOverrides(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		return fmt.Errorf("rpcids: %s: %w", path, err)
	}
	var unknown []string
	for name := range m {
		if _, ok := defaults[name]; !ok {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) > 0 {
		slices.Sort(unknown)
		return fmt.Errorf("rpcids: %s: unknown names: %s", path, strings.Join(unknown, ", "))
	}
	mu.Lock()
	defer mu.Unlock()
	for name, id := range m {
		ids[name] = id
	}
	return nil
}
