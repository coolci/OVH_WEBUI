package ovh

import (
	"fmt"
	"net/url"
	"strings"
)

// BuildPath constructs a safe, URL-escaped OVH API path from individual path segments.
// Every segment must be non-empty, must not be "." or ".." (including percent-encoded dot segments),
// must not contain ASCII control characters, and is escaped via url.PathEscape.
func BuildPath(segments ...string) (string, error) {
	if len(segments) == 0 {
		return "", fmt.Errorf("ovh.BuildPath: at least one path segment is required")
	}
	var b strings.Builder
	for i, seg := range segments {
		if seg == "" {
			return "", fmt.Errorf("ovh.BuildPath: segment %d is empty", i)
		}
		if isDotTraversalSegment(seg) {
			return "", fmt.Errorf("ovh.BuildPath: dot traversal segment %q is forbidden", seg)
		}
		for j := 0; j < len(seg); j++ {
			if seg[j] < 0x20 || seg[j] == 0x7f {
				return "", fmt.Errorf("ovh.BuildPath: segment %d contains control character", i)
			}
		}
		b.WriteByte('/')
		b.WriteString(url.PathEscape(seg))
	}
	return b.String(), nil
}

func isDotTraversalSegment(seg string) bool {
	if seg == "." || seg == ".." {
		return true
	}
	unescaped, err := url.PathUnescape(seg)
	if err == nil && (unescaped == "." || unescaped == "..") {
		return true
	}
	return false
}

// WithQuery appends URL-encoded query parameters to basePath.
// When q is empty, basePath is returned unchanged.
func WithQuery(basePath string, q url.Values) string {
	if len(q) == 0 {
		return basePath
	}
	encoded := q.Encode()
	if encoded == "" {
		return basePath
	}
	if strings.Contains(basePath, "?") {
		if strings.HasSuffix(basePath, "?") || strings.HasSuffix(basePath, "&") {
			return basePath + encoded
		}
		return basePath + "&" + encoded
	}
	return basePath + "?" + encoded
}
