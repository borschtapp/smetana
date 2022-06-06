package config

var Env struct {
	JwtSecretKey            string
	JwtRefreshKey           string
	JwtTokenExpireMinutes   uint `envconfig:"default=4320"`  // 3 days
	JwtRefreshExpireMinutes uint `envconfig:"default=10080"` // 7 days

	Database struct {
		Name     string `envconfig:"optional"`
		Host     string `envconfig:"optional"`
		Port     uint16 `envconfig:"optional"`
		Username string `envconfig:"optional"`
		Password string `envconfig:"optional"`
	}
}
