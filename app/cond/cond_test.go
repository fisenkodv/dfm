package cond

import "testing"

func TestEval(t *testing.T) {
	ctx := Context{OS: "darwin", Arch: "arm64", Hostname: "mbp"}
	tests := []struct {
		expr string
		want bool
		bad  bool
	}{
		{"", true, false},
		{`os == "darwin"`, true, false},
		{`os == "linux"`, false, false},
		{`os != "linux"`, true, false},
		{`os == "darwin" && arch == "arm64"`, true, false},
		{`os == "darwin" && arch == "amd64"`, false, false},
		{`os == "linux" || os == "darwin"`, true, false},
		{`!(os == "linux")`, true, false},
		{`hostname == "mbp"`, true, false},
		{`hostname == "other"`, false, false},
		{`(os == "darwin") && (hostname == "mbp")`, true, false},
		{`unknownvar == "x"`, false, false},
		{`  os  ==  "darwin"  `, true, false},
		// errors
		{`os ==`, false, true},
		{`os == "darwin" &&`, false, true},
		{`(os == "darwin"`, false, true},
		{`os = "darwin"`, false, true},
	}
	for _, tt := range tests {
		got, err := Eval(tt.expr, ctx)
		if tt.bad {
			if err == nil {
				t.Errorf("Eval(%q): want error, got %v", tt.expr, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("Eval(%q): unexpected err: %v", tt.expr, err)
			continue
		}
		if got != tt.want {
			t.Errorf("Eval(%q) = %v, want %v", tt.expr, got, tt.want)
		}
	}
}
