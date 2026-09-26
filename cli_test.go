package main

import "testing"

func TestParseArgsOutputDirectory(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "defaults beside input file",
			args: []string{"docs/rfc.md"},
			want: "docs/.quorum/runs",
		},
		{
			name: "out overrides default",
			args: []string{"--out", "../reviews", "docs/rfc.md"},
			want: "../reviews",
		},
		{
			name: "output-dir alias overrides default",
			args: []string{"--output-dir", "/tmp/reviews", "docs/rfc.md"},
			want: "/tmp/reviews",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o, err := parseArgs(tt.args)
			if err != nil {
				t.Fatalf("parseArgs() error = %v", err)
			}
			if o.Out != tt.want {
				t.Errorf("output directory = %q, want %q", o.Out, tt.want)
			}
		})
	}
}
