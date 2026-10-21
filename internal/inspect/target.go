package inspect

import (
	"errors"
	"strings"
)

// parseTarget parses session URI or shorthand into harness and id components.
// Accepted forms:
// - deja://session/<harness>:<id>
// - deja://session/<id>
// - <harness>:<id>
// - <id> (bare ID)
func parseTarget(raw string) (TargetInfo, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return TargetInfo{}, errors.New("empty target")
	}

	info := TargetInfo{Raw: raw}
	s := trimmed

	// Strip deja://session/ URI scheme variants
	if after, ok := strings.CutPrefix(s, "deja://session/"); ok {
		s = after
	} else if after, ok := strings.CutPrefix(s, "deja://session"); ok {
		s = strings.TrimPrefix(after, "/")
	} else if after, ok := strings.CutPrefix(s, "deja:///session/"); ok {
		s = after
	}

	// Remove any trailing slashes
	s = strings.TrimRight(s, "/")

	if idx := strings.Index(s, ":"); idx >= 0 {
		info.Harness = strings.ToLower(s[:idx])
		info.ID = s[idx+1:]
	} else {
		info.ID = s
	}

	return info, nil
}
