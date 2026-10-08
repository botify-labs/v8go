package v8go_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	v8 "github.com/botify-labs/v8go"
)

// The default locale is process-wide, so the test runs in a child process
// whose host locale is French.
func TestSetDefaultLocale(t *testing.T) {
	if locale := os.Getenv("V8GO_TEST_SET_DEFAULT_LOCALE"); locale != "" {
		if err := v8.SetDefaultLocale(locale); err != nil {
			t.Fatal(err)
		}
		iso := v8.NewIsolate()
		defer iso.Dispose()
		ctx := v8.NewContext(iso)
		defer ctx.Close()
		for script, want := range map[string]string{
			"(1234567.891).toLocaleString()":                               "1,234,567.891",
			"new Intl.NumberFormat().resolvedOptions().locale":             "en-US",
			"new Date(0).toLocaleDateString(undefined, {timeZone: 'UTC'})": "1/1/1970",
		} {
			val, err := ctx.RunScript(script, "locale.js")
			if err != nil {
				t.Fatalf("%s: %v", script, err)
			}
			if got := val.String(); got != want {
				t.Errorf("%s = %q, want %q", script, got, want)
			}
		}
		return
	}

	// nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	cmd := exec.Command(os.Args[0], "-test.run=^TestSetDefaultLocale$", "-test.v")
	cmd.Env = append(os.Environ(),
		"V8GO_TEST_SET_DEFAULT_LOCALE=en_US",
		"LANG=fr_FR.UTF-8", "LC_ALL=fr_FR.UTF-8", "LC_MESSAGES=fr_FR.UTF-8")
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "--- PASS: TestSetDefaultLocale") {
		t.Fatalf("child process: %v\n%s", err, out)
	}
}
