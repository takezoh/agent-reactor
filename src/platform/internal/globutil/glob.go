package globutil

import (
	"regexp"
	"strings"
)

// CompileGlob converts a glob pattern (only * is special) to a regex.
// * matches any string including spaces; literal characters are escaped.
func CompileGlob(pattern string) (*regexp.Regexp, error) {
	parts := strings.Split(pattern, "*")
	quoted := make([]string, len(parts))
	for i, p := range parts {
		quoted[i] = regexp.QuoteMeta(p)
	}
	return regexp.Compile(`(?s)\A` + strings.Join(quoted, `.*`) + `\z`)
}
