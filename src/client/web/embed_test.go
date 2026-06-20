package web

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

// TestEmbedHasNoInlineScript ensures the embedded dist/index.html does not
// contain inline <script>...</script> blocks or unsafe-inline directives.
// Regression guard for FR-β13 / FR-β03.
//
// This test checks the *static file content* embedded in the binary; CSP
// header values are asserted separately by TestCSPHeaders in headers_test.go.
func TestEmbedHasNoInlineScript(t *testing.T) {
	t.Parallel()

	sub, err := fs.Sub(Assets, "dist")
	if err != nil {
		t.Fatalf("sub fs: %v", err)
	}
	b, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	html := string(b)

	if strings.Contains(html, "unsafe-inline") {
		t.Errorf("dist/index.html must not contain unsafe-inline, got:\n%s", html)
	}

	// Detect inline <script>...</script>: any script tag whose body is
	// non-empty is an inline script. External-only tags look like
	//   <script type="module" src="..."></script>
	// and have an empty body, which is fine.
	inline := regexp.MustCompile(`(?i)<script(?:\s[^>]*)?>`)
	for _, tag := range inline.FindAllStringIndex(html, -1) {
		tagEnd := tag[1]
		// Find the closing </script>
		closeTag := strings.Index(strings.ToLower(html[tagEnd:]), "</script>")
		if closeTag < 0 {
			t.Errorf("dist/index.html has unclosed <script> tag at offset %d", tag[0])
			continue
		}
		body := html[tagEnd : tagEnd+closeTag]
		if strings.TrimSpace(body) != "" {
			t.Errorf("dist/index.html must not contain inline script body, got: %q",
				html[tag[0]:tagEnd+closeTag+len("</script>")])
		}
	}
}
