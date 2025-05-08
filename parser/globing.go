package parser

import (
	"path/filepath"
	"strings"
)

// TODO: Implement this without using filepath.Glob.
// TODO: Implement support for {}.
func evalGlobing(in string) string {
	matches, err := filepath.Glob(in)
	if err != nil || len(matches) == 0 {
		return in
	}
	return strings.Join(matches, " ")
}
