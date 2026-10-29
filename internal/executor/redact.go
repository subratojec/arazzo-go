package executor

import (
	"encoding/json"
	"net/url"
	"strings"
)

var secretKeyFragments = []string{
	"password",
	"passwd",
	"secret",
	"token",
	"session",
	"totp",
	"otp",
	"credential",
	"authorization",
	"cookie",
}

func isSecretKey(key string) bool {
	k := strings.ToLower(key)
	for _, frag := range secretKeyFragments {
		if strings.Contains(k, frag) {
			return true
		}
	}
	return false
}

// RedactValue recursively walks maps and slices, masking values whose keys match secret patterns.
func RedactValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			if isSecretKey(k) {
				out[k] = "***"
				continue
			}
			out[k] = RedactValue(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = RedactValue(val)
		}
		return out
	default:
		return v
	}
}

func redactValue(v any) any {
	return RedactValue(v)
}

func redactJSON(v any) string {
	b, err := json.Marshal(RedactValue(v))
	if err != nil {
		return "<unserializable>"
	}
	return string(b)
}

func truncateForLog(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	changed := false
	for k := range q {
		if isSecretKey(k) {
			q.Set(k, "***")
			changed = true
		}
	}
	if !changed {
		return raw
	}
	u.RawQuery = q.Encode()
	return u.String()
}
