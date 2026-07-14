package sqliteutil

import "testing"

func TestDSN(t *testing.T) {
	tests := []struct {
		name  string
		build func(string) string
		input string
		want  string
	}{
		{
			name:  "filesystem path",
			build: DSN,
			input: "/var/lib/tdns/tdns.sqlite",
			want:  "file:/var/lib/tdns/tdns.sqlite?cache=shared",
		},
		{
			name:  "SQLite file URL",
			build: DSN,
			input: "file:tdns.sqlite",
			want:  "file:tdns.sqlite?cache=shared",
		},
		{
			name:  "existing query parameters",
			build: DSN,
			input: "file:tdns.sqlite?cache=private&_pragma=foreign_keys(1)",
			want:  "file:tdns.sqlite?cache=private&_pragma=foreign_keys(1)",
		},
		{
			name:  "read-only connection",
			build: ReadOnlyDSN,
			input: "/var/lib/tdns/tdns.sqlite",
			want:  "file:/var/lib/tdns/tdns.sqlite?cache=shared&mode=ro",
		},
		{
			name:  "read-write connection",
			build: ReadWriteDSN,
			input: "file:tdns.sqlite?cache=private",
			want:  "file:tdns.sqlite?cache=private&mode=rwc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.build(tt.input); got != tt.want {
				t.Fatalf("DSN = %q, want %q", got, tt.want)
			}
		})
	}
}
