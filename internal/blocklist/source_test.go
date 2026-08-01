package blocklist

import "testing"

func TestParseSource(t *testing.T) {
	source, err := ParseSource("https://github.com/StevenBlack/hosts.git", "feature/lists", "alternates/gambling/hosts")
	if err != nil {
		t.Fatalf("ParseSource error: %v", err)
	}
	if source.Owner != "StevenBlack" || source.Repo != "hosts" || source.Branch != "feature/lists" || source.File != "alternates/gambling/hosts" {
		t.Fatalf("unexpected source: %#v", source)
	}
}

func TestParseSourceRejectsUntrustedComponents(t *testing.T) {
	tests := []struct {
		name       string
		repository string
		branch     string
		file       string
	}{
		{"http repository", "http://github.com/acme/lists", "main", "hosts"},
		{"credentials", "https://token@github.com/acme/lists", "main", "hosts"},
		{"unexpected host", "https://github.example/acme/lists", "main", "hosts"},
		{"extra repository path", "https://github.com/acme/lists/extra", "main", "hosts"},
		{"encoded owner", "https://github.com/acme%2Flists/repo", "main", "hosts"},
		{"branch traversal", "https://github.com/acme/lists", "../main", "hosts"},
		{"branch ref syntax", "https://github.com/acme/lists", "main@{1}", "hosts"},
		{"hidden branch component", "https://github.com/acme/lists", "feature/.hidden", "hosts"},
		{"locked branch component", "https://github.com/acme/lists", "feature.lock/main", "hosts"},
		{"absolute file", "https://github.com/acme/lists", "main", "/hosts"},
		{"file traversal", "https://github.com/acme/lists", "main", "alternate/../hosts"},
		{"backslash file", "https://github.com/acme/lists", "main", `alternate\hosts`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ParseSource(test.repository, test.branch, test.file); err == nil {
				t.Fatal("expected source validation error")
			} else if KindOf(err) != KindInvalid {
				t.Fatalf("KindOf(error) = %q, want %q", KindOf(err), KindInvalid)
			}
		})
	}
}
