package android

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

// IssuesURL is where Android bug reports go: upstream's issues, like the PC
// GUI's.
const IssuesURL = "https://github.com/bedrock-tool/bedrocktool/issues/new"

// logLines is how much of the log a report carries: enough to see what led
// up to a problem, small enough for the clipboard and a GitHub issue.
const logLines = 300

func getprop(name string) string {
	out, err := exec.Command("/system/bin/getprop", name).Output()
	if err != nil {
		return "?"
	}
	return strings.TrimSpace(string(out))
}

func tail(path string, n int) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimRight(utils.StripAnsi(string(b)), "\n"), "\n")
	if len(lines) > n {
		lines = append([]string{fmt.Sprintf("[... %d earlier lines left out]", len(lines)-n)}, lines[len(lines)-n:]...)
	}
	return strings.Join(lines, "\n")
}

// newestRecoveredCrash is the newest crash-*.txt, which utils.PrintPanic
// writes for a panic the GUI caught (and survived).
func newestRecoveredCrash() string {
	files, _ := filepath.Glob(utils.PathData("crash-2*.txt"))
	if len(files) == 0 {
		return ""
	}
	sort.Strings(files) // their names are timestamps
	b, _ := os.ReadFile(files[len(files)-1])
	return string(b)
}

func section(b *strings.Builder, title, body string) {
	if strings.TrimSpace(body) == "" {
		return
	}
	fmt.Fprintf(b, "\n### %s\n\n```\n%s\n```\n", title, strings.TrimSpace(body))
}

// Report is the text the user pastes into a GitHub issue: versions, the
// phone, the last crash and the end of the log.
func Report() string {
	var b strings.Builder
	fmt.Fprintf(&b, "**Bedrocktool** %s (Android), Minecraft %s\n", utils.Version, protocol.CurrentVersion)
	fmt.Fprintf(&b, "**Phone** %s %s, Android %s (API %s), %s\n",
		getprop("ro.product.manufacturer"), getprop("ro.product.model"),
		getprop("ro.build.version.release"), getprop("ro.build.version.sdk"),
		getprop("ro.product.cpu.abi"))

	crash := LastCrash()
	section(&b, "Crash (previous run)", crash)
	section(&b, "Last caught error", newestRecoveredCrash())
	if crash != "" {
		section(&b, "Log of the run that crashed", tail(utils.PathData(prevLogFile), logLines))
	}
	section(&b, "Log", tail(utils.PathData(logFile), logLines))
	return b.String()
}

// IssueURL opens a new issue with a title and a body that asks for the
// report; the report itself is too long for a URL, so it goes on the
// clipboard.
func IssueURL(crashed bool) string {
	title := T("android_issue_title", nil)
	if crashed {
		title = T("android_issue_title_crash", nil)
	}
	q := url.Values{}
	q.Set("title", title)
	q.Set("body", T("android_issue_body", nil))
	return IssuesURL + "?" + q.Encode()
}
