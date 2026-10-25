package configs

import (
	"strings"
	"testing"
	"time"

	gomysql "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
)

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

func TestMySQLDSN(t *testing.T) {
	testCases := map[string]struct {
		cfg      databaseEnvVars
		wantAddr string
	}{
		"default port":       {cfg: databaseEnvVars{Host: "db", User: "user", Password: "password", Name: "smetana"}, wantAddr: "db:3306"},
		"custom port":        {cfg: databaseEnvVars{Host: "db", Port: 3307, User: "user", Password: "password", Name: "smetana"}, wantAddr: "db:3307"},
		"ipv6 host":          {cfg: databaseEnvVars{Host: "::1", Port: 3307, User: "user", Password: "password", Name: "smetana"}, wantAddr: "[::1]:3307"},
		"special characters": {cfg: databaseEnvVars{Host: "db", User: "us@er", Password: "p@ss/w:rd", Name: "smetana"}, wantAddr: "db:3306"},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			dsn := mysqlDSN(tc.cfg)
			parsed, err := gomysql.ParseDSN(dsn)
			if err != nil {
				t.Fatalf("ParseDSN(%q): %v", dsn, err)
			}
			if parsed.Addr != tc.wantAddr || parsed.User != tc.cfg.User || parsed.Passwd != tc.cfg.Password || parsed.DBName != tc.cfg.Name {
				t.Errorf("mysqlDSN(%+v) = %q, parsed as addr=%q user=%q password=%q db=%q",
					tc.cfg, dsn, parsed.Addr, parsed.User, parsed.Passwd, parsed.DBName)
			}
			if !parsed.ClientFoundRows {
				t.Errorf("mysqlDSN(%+v) = %q, want clientFoundRows=true", tc.cfg, dsn)
			}
			if !parsed.ParseTime || parsed.Loc != time.Local {
				t.Errorf("mysqlDSN(%+v) = %q, want parseTime=true and loc=Local", tc.cfg, dsn)
			}
			if !strings.Contains(dsn, "charset=utf8mb4") {
				t.Errorf("mysqlDSN(%+v) = %q, want charset=utf8mb4", tc.cfg, dsn)
			}
		})
	}
}

func TestPostgresDSN(t *testing.T) {
	testCases := map[string]struct {
		cfg  databaseEnvVars
		want string
	}{
		"default port": {
			cfg:  databaseEnvVars{Host: "localhost", User: "user", Password: "password", Name: "smetana", SSLMode: "disable"},
			want: "postgres://user:password@localhost:5432/smetana?TimeZone=UTC&sslmode=disable",
		},
		"custom port": {
			cfg:  databaseEnvVars{Host: "localhost", Port: 5433, User: "user", Password: "password", Name: "smetana", SSLMode: "disable"},
			want: "postgres://user:password@localhost:5433/smetana?TimeZone=UTC&sslmode=disable",
		},
		"empty password": {
			cfg:  databaseEnvVars{Host: "localhost", User: "user", Name: "smetana", SSLMode: "disable"},
			want: "postgres://user@localhost:5432/smetana?TimeZone=UTC&sslmode=disable",
		},
		"empty credentials": {
			cfg:  databaseEnvVars{Host: "localhost", Name: "smetana", SSLMode: "disable"},
			want: "postgres://localhost:5432/smetana?TimeZone=UTC&sslmode=disable",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			if got := postgresDSN(tc.cfg); got != tc.want {
				t.Errorf("postgresDSN(%+v) = %q, want %q", tc.cfg, got, tc.want)
			}
		})
	}
}

func TestPostgresDSNRoundTrip(t *testing.T) {
	testCases := map[string]databaseEnvVars{
		"empty password":     {Host: "db", User: "smetana", Name: "smetana", SSLMode: "disable"},
		"special characters": {Host: "db", User: "us er", Password: "p@ss w/rd'\\", Name: "my db", SSLMode: "require"},
		"ipv6 host":          {Host: "::1", Port: 5433, User: "u", Password: "p", Name: "smetana", SSLMode: "disable"},
	}

	for name, cfg := range testCases {
		t.Run(name, func(t *testing.T) {
			parsed, err := pgconn.ParseConfig(postgresDSN(cfg))
			if err != nil {
				t.Fatalf("ParseConfig: %v", err)
			}
			wantPort := uint16(5432)
			if cfg.Port > 0 {
				wantPort = uint16(cfg.Port)
			}
			if parsed.Host != cfg.Host || parsed.Port != wantPort || parsed.User != cfg.User ||
				parsed.Password != cfg.Password || parsed.Database != cfg.Name {
				t.Errorf("round trip mismatch: host=%q port=%d user=%q password=%q db=%q",
					parsed.Host, parsed.Port, parsed.User, parsed.Password, parsed.Database)
			}
			if tz := parsed.RuntimeParams["TimeZone"]; tz != "UTC" {
				t.Errorf("TimeZone = %q, want UTC", tz)
			}
		})
	}
}
