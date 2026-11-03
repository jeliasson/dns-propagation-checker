package cli

import (
	"bytes"
	"strings"
	"testing"
)

func run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestRunNoArgsPrintsUsage(t *testing.T) {
	code, _, stderr := run(t)
	if code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr, "Usage:") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestRunVersion(t *testing.T) {
	for _, arg := range []string{"version", "--version", "-version"} {
		code, stdout, _ := run(t, arg)
		if code != ExitOK {
			t.Errorf("%s: exit = %d", arg, code)
		}
		if !strings.Contains(stdout, "dnspc") {
			t.Errorf("%s: stdout = %q", arg, stdout)
		}
	}
}

func TestRunHelp(t *testing.T) {
	code, stdout, _ := run(t, "help")
	if code != ExitOK || !strings.Contains(stdout, "dnspc check") {
		t.Errorf("exit = %d, stdout = %q", code, stdout)
	}
}

func TestRunCheckWithNoNameIsAUsageError(t *testing.T) {
	code, _, stderr := run(t, "check")
	if code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr, "no name to check") || !strings.Contains(stderr, "Usage:") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestRunCheckRejectsInvalidInput(t *testing.T) {
	code, _, stderr := run(t, "check", "a;rm -rf /", "@127.0.0.1:1")
	if code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr, "invalid name") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestRunCheckUnreachableServerReportsPerAnswer(t *testing.T) {
	// A server that will not answer is a result, not a crash: exit 0 unless
	// -fail-on-diff was asked for.
	code, stdout, stderr := run(t,
		"check", "example.com", "@127.0.0.1:1", "-timeout", "200ms")
	if code != ExitOK {
		t.Errorf("exit = %d, want %d (stderr %q)", code, ExitOK, stderr)
	}
	if !strings.Contains(stdout, "example.com A") {
		t.Errorf("stdout = %q", stdout)
	}
}

func TestRunCheckFailOnDiffExitsTwo(t *testing.T) {
	code, _, _ := run(t,
		"check", "example.com", "@127.0.0.1:1", "-timeout", "200ms", "-fail-on-diff")
	if code != ExitDiff {
		t.Errorf("exit = %d, want %d", code, ExitDiff)
	}
}

func TestRunCheckJSONOutput(t *testing.T) {
	_, stdout, _ := run(t,
		"check", "example.com", "@127.0.0.1:1", "-timeout", "200ms", "-json")
	for _, want := range []string{`"records"`, `"fqdn": "example.com"`, `"answers"`} {
		if !strings.Contains(stdout, want) {
			t.Errorf("json output missing %q:\n%s", want, stdout)
		}
	}
}

func TestRunBareNameImpliesCheck(t *testing.T) {
	code, stdout, _ := run(t, "example.com", "@127.0.0.1:1", "-timeout", "200ms")
	if code != ExitOK {
		t.Errorf("exit = %d", code)
	}
	if !strings.Contains(stdout, "example.com") {
		t.Errorf("stdout = %q", stdout)
	}
}

func TestRunCheckHelpExitsZero(t *testing.T) {
	code, _, stderr := run(t, "check", "-h")
	if code != ExitOK {
		t.Errorf("exit = %d, want %d", code, ExitOK)
	}
	if !strings.Contains(stderr, "fail-on-diff") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestRunServeRejectsBadFlags(t *testing.T) {
	if code, _, _ := run(t, "serve", "-log-level", "chatty"); code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if code, _, _ := run(t, "serve", "-nope"); code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
}

func TestParseLevel(t *testing.T) {
	if _, err := parseLevel("debug"); err != nil {
		t.Errorf("debug should parse: %v", err)
	}
	if _, err := parseLevel("nonsense"); err == nil {
		t.Error("nonsense should not parse")
	}
}

func TestEnvOr(t *testing.T) {
	t.Setenv("DNSPC_TEST_VALUE", "set")
	if got := envOr("DNSPC_TEST_VALUE", "fallback"); got != "set" {
		t.Errorf("envOr = %q", got)
	}
	if got := envOr("DNSPC_TEST_UNSET", "fallback"); got != "fallback" {
		t.Errorf("envOr = %q", got)
	}
}

func TestPaint(t *testing.T) {
	if got := paint("x", red, false); got != "x" {
		t.Errorf("paint disabled = %q", got)
	}
	if got := paint("x", red, true); got != "\x1b[31mx\x1b[0m" {
		t.Errorf("paint enabled = %q", got)
	}
}

func TestUseColorOnNonFile(t *testing.T) {
	if useColor(&bytes.Buffer{}) {
		t.Error("a buffer is not a terminal")
	}
}

func TestUseColorHonoursNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if useColor(nil) {
		t.Error("NO_COLOR must disable color")
	}
}
