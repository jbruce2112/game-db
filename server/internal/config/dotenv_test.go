package config

import "testing"

func TestParseDotEnvLine(t *testing.T) {
	cases := []struct {
		in      string
		key     string
		val     string
		ok      bool
		wantErr bool
	}{
		{"", "", "", false, false},
		{"# comment", "", "", false, false},
		{"APP_PASSWORD=secret", "APP_PASSWORD", "secret", true, false},
		{"APP_PASSWORD=secret # inline", "APP_PASSWORD", "secret", true, false},
		{`APP_PASSWORD="secret # still"`, "APP_PASSWORD", "secret # still", true, false},
		{"export DATA_DIR=../data", "DATA_DIR", "../data", true, false},
		{"NOEQUALS", "", "", false, true},
	}
	for _, c := range cases {
		k, v, ok, err := parseDotEnvLine(c.in)
		if c.wantErr {
			if err == nil {
				t.Fatalf("%q: expected error", c.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%q: %v", c.in, err)
		}
		if ok != c.ok || k != c.key || v != c.val {
			t.Fatalf("%q: got %v %q=%q", c.in, ok, k, v)
		}
	}
}
