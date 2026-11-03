package cli

import (
	"flag"
	"io"
	"strings"
	"testing"
)

func testFlagSet() (*flag.FlagSet, *bool, *string) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	verbose := fs.Bool("json", false, "")
	timeout := fs.String("timeout", "", "")
	return fs, verbose, timeout
}

func TestReorderArgs(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "flags after positionals are hoisted",
			in:   []string{"example.com", "--json", "@1.1.1.1"},
			want: []string{"--json", "example.com", "@1.1.1.1"},
		},
		{
			name: "value flags keep their value",
			in:   []string{"example.com", "-timeout", "2s", "@1.1.1.1"},
			want: []string{"-timeout", "2s", "example.com", "@1.1.1.1"},
		},
		{
			name: "inline values are left alone",
			in:   []string{"example.com", "-timeout=2s"},
			want: []string{"-timeout=2s", "example.com"},
		},
		{
			name: "already ordered input is unchanged",
			in:   []string{"-json", "example.com"},
			want: []string{"-json", "example.com"},
		},
		{
			name: "double dash ends flag parsing",
			in:   []string{"-json", "--", "-weird.example.com"},
			want: []string{"-json", "-weird.example.com"},
		},
		{
			name: "unknown flags are preserved for the parser to reject",
			in:   []string{"example.com", "-nope"},
			want: []string{"-nope", "example.com"},
		},
		{
			name: "no args",
			in:   nil,
			want: nil,
		},
	}

	for _, tc := range tests {
		fs, _, _ := testFlagSet()
		got := reorderArgs(fs, tc.in)
		if strings.Join(got, "\x00") != strings.Join(tc.want, "\x00") {
			t.Errorf("%s: reorderArgs = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestReorderArgsThenParse(t *testing.T) {
	fs, asJSON, timeout := testFlagSet()
	args := []string{"example.com", "@1.1.1.1", "-json", "-timeout", "2s"}

	if err := fs.Parse(reorderArgs(fs, args)); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !*asJSON {
		t.Error("-json was not picked up")
	}
	if *timeout != "2s" {
		t.Errorf("-timeout = %q, want 2s", *timeout)
	}
	if got := strings.Join(fs.Args(), ","); got != "example.com,@1.1.1.1" {
		t.Errorf("positionals = %q", got)
	}
}
