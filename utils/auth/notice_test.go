package auth

import (
	"strings"
	"testing"
)

// The CLI prints the #348 note before the login code.
func TestCliAuthShowsNoticeFirst(t *testing.T) {
	var out strings.Builder
	newCliAuthCodeHandler(&out).DisplayCode("https://www.microsoft.com/link", "ABCD1234")
	s := out.String()
	notice := strings.Index(s, AntiCheatNotice())
	code := strings.Index(s, "ABCD1234")
	if notice < 0 || code < 0 || notice > code {
		t.Fatalf("want the notice before the code, got:\n%s", s)
	}
}
