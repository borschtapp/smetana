package configs

import (
	"os"

	"borscht.app/smetana/internal/utils"
)

// JwtSecretKey returns the secret key for JWT.
func JwtSecretKey() []byte {
	return []byte(os.Getenv("JWT_SECRET_KEY"))
}

// JwtSecretExpireMinutes returns the expiration time for JWT in minutes.
func JwtSecretExpireMinutes() int {
	return utils.GetenvInt("JWT_SECRET_EXPIRE_MINUTES", 60)
}

// JwtRefreshExpireMinutes returns the expiration time for JWT refresh token in minutes.
func JwtRefreshExpireMinutes() int {
	return utils.GetenvInt("JWT_REFRESH_EXPIRE_MINUTES", 10080) // 7 days
}
