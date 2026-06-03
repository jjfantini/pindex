// Package envfile loads KEY=VALUE pairs from a .env file into the process
// environment. It exists so freshly-edited secrets in a gitignored .env take
// effect even when the surrounding shell environment is stale.
package envfile

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Load reads path and sets each KEY=VALUE in the process environment, OVERRIDING
// existing values so an updated .env wins over an inherited environment. A
// missing file is a no-op. Blank lines and # comments are ignored; an optional
// leading "export " and surrounding single/double quotes are stripped.
func Load(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer func() { _ = f.Close() }()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		if err := os.Setenv(k, v); err != nil {
			return fmt.Errorf("envfile: set %s: %w", k, err)
		}
	}
	return sc.Err()
}
