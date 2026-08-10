package configs

import "testing"

func TestSQLiteDSNEnablesForeignKeys(t *testing.T) {
	testCases := map[string]struct {
		dsn  string
		want string
	}{
		"file path": {
			dsn:  "./data/borscht.db",
			want: "./data/borscht.db?_foreign_keys=on",
		},
		"existing parameters": {
			dsn:  "file:test.db?mode=memory&cache=shared",
			want: "file:test.db?_foreign_keys=on&cache=shared&mode=memory",
		},
		"disabled foreign keys are overridden": {
			dsn:  "file:test.db?_foreign_keys=off&cache=shared",
			want: "file:test.db?_foreign_keys=on&cache=shared",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			if got := sqliteDSN(tc.dsn); got != tc.want {
				t.Errorf("sqliteDSN(%q) = %q, want %q", tc.dsn, got, tc.want)
			}
		})
	}
}
