package configs

import (
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	gomysql "github.com/go-sql-driver/mysql"
	"github.com/gofiber/fiber/v3/log"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"borscht.app/smetana/internal/utils"
)

type databaseEnvVars struct {
	Type     string
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string // For Postgres
}

func mysqlDSN(cfg databaseEnvVars) string {
	port := cfg.Port
	if port <= 0 {
		port = 3306
	}
	dsn := gomysql.NewConfig()
	dsn.User = cfg.User
	dsn.Passwd = cfg.Password
	dsn.Net = "tcp"
	dsn.Addr = net.JoinHostPort(cfg.Host, strconv.Itoa(port))
	dsn.DBName = cfg.Name
	dsn.ParseTime = true
	dsn.Loc = time.Local
	dsn.ClientFoundRows = true // report matched, not changed, rows like SQLite/Postgres; a no-op UPDATE isn't "not found"
	_ = dsn.Apply(gomysql.Charset("utf8mb4", ""))
	return dsn.FormatDSN()
}

// postgresDSN builds a postgres:// URL so that credentials and names are
// percent-encoded; the key/value form breaks on empty or space-containing values.
func postgresDSN(cfg databaseEnvVars) string {
	port := cfg.Port
	if port <= 0 {
		port = 5432
	}
	u := url.URL{
		Scheme:   "postgres",
		Host:     net.JoinHostPort(cfg.Host, strconv.Itoa(port)),
		Path:     "/" + cfg.Name,
		RawQuery: url.Values{"sslmode": {cfg.SSLMode}, "TimeZone": {"UTC"}}.Encode(),
	}
	if cfg.Password != "" {
		u.User = url.UserPassword(cfg.User, cfg.Password)
	} else if cfg.User != "" {
		u.User = url.User(cfg.User)
	}
	return u.String()
}

// sqliteDSN ensures that SQLite enforces foreign-key constraints on every
// connection opened by the driver. Foreign keys are disabled by default in
// SQLite, even when the schema contains FOREIGN KEY clauses.
func sqliteDSN(dsn string) string {
	base, query, hasQuery := strings.Cut(dsn, "?")
	if !hasQuery {
		return dsn + "?_foreign_keys=on"
	}

	values, err := url.ParseQuery(query)
	if err != nil {
		// Preserve an otherwise valid driver DSN and still request enforcement.
		return dsn + "&_foreign_keys=on"
	}
	// url.Values.Get returns the first matching value. Remove any supplied
	// setting so DB_NAME cannot accidentally leave foreign keys disabled.
	values.Del("_foreign_keys")
	values.Set("_foreign_keys", "on")
	return base + "?" + values.Encode()
}

var dialectRegistry = map[string]func(databaseEnvVars) gorm.Dialector{
	"mysql": func(cfg databaseEnvVars) gorm.Dialector {
		return mysql.Open(mysqlDSN(cfg))
	},
	"postgres": func(cfg databaseEnvVars) gorm.Dialector {
		return postgres.Open(postgresDSN(cfg))
	},
	"sqlite": func(cfg databaseEnvVars) gorm.Dialector {
		_ = os.MkdirAll(filepath.Dir(cfg.Name), 0700)
		return sqlite.Open(sqliteDSN(cfg.Name))
	},
}

func GormConfig() (gorm.Config, gorm.Dialector) {
	enableLogger := utils.GetenvBool("LOG_DB_QUERIES", false)
	envVars := databaseEnvVars{
		Type:     utils.Getenv("DB_TYPE", "sqlite"),
		Host:     utils.Getenv("DB_HOST", "localhost"),
		Port:     utils.GetenvInt("DB_PORT", 0),
		Name:     utils.Getenv("DB_NAME", "./data/borscht.db"),
		User:     utils.Getenv("DB_USER", ""),
		Password: utils.Getenv("DB_PASSWORD", ""),
		SSLMode:  utils.Getenv("DB_SSLMODE", "disable"),
	}

	config := gorm.Config{
		TranslateError: true,
	}
	if enableLogger {
		config.Logger = logger.Default.LogMode(logger.Info)
	}

	factory, ok := dialectRegistry[envVars.Type]
	if !ok {
		log.Fatalw("unsupported DB_TYPE", "type", envVars.Type, "supported", "sqlite, mysql, postgres")
	}
	log.Infow("Database initialized", "type", envVars.Type, "host", envVars.Host, "name", envVars.Name)
	return config, factory(envVars)
}
