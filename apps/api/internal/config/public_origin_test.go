package config

import "testing"

func TestPublicManagerOrigin(t *testing.T) {
	for _, tt := range []struct {
		input, want string
		bad         bool
	}{
		{"", "", false}, {"https://p3express.avari.dev/", "https://p3express.avari.dev", false},
		{"http://p3express.avari.dev", "", true}, {"https://user@p3express.avari.dev", "", true},
		{"https://p3express.avari.dev/path", "", true}, {"https://p3express.avari.dev?", "", true},
		{"https://p3express.avari.dev#", "", true}, {"https://p3express.avari.dev,https://other.dev", "", true},
	} {
		t.Run(tt.input, func(t *testing.T) {
			got, err := NormalizePublicManagerOrigin(tt.input)
			if (err != nil) != tt.bad || got != tt.want {
				t.Fatalf("got %q, %v", got, err)
			}
		})
	}
}

func TestLoadPublicManagerOrigin(t *testing.T) {
	t.Setenv("PUBLIC_MANAGER_ORIGIN", "https://p3express.avari.dev/")
	cfg, err := Load()
	if err != nil || cfg.PublicManagerOrigin != "https://p3express.avari.dev" {
		t.Fatalf("config %+v, %v", cfg, err)
	}
	t.Setenv("PUBLIC_MANAGER_ORIGIN", "https://p3express.avari.dev/path")
	if _, err := Load(); err == nil {
		t.Fatal("invalid origin accepted")
	}
}
