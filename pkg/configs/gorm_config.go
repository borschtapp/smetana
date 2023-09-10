package configs

import (
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"borscht.app/smetana/pkg/utils"
)

func GormConfig() gorm.Config {
	enableLogger := utils.GetenvBool("GORM_ENABLE_LOGGER", false)

	config := gorm.Config{}
	if enableLogger {
		config.Logger = logger.Default.LogMode(logger.Info)
	}

	return config
}
