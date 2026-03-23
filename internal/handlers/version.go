package handlers

import (
	"github.com/gofiber/fiber/v3"
)

type versionResponse struct {
	Current string `json:"current"`
}

// NewVersionHandler godoc
// @Summary Show the app version.
// @Description get the current app version and latest available version from GitHub.
// @Tags root
// @Accept */*
// @Produce json
// @Success 200 {object} versionResponse
// @Router /_version [get]
func NewVersionHandler(version string) fiber.Handler {
	return func(c fiber.Ctx) error {
		return c.JSON(versionResponse{
			Current: version,
		})
	}
}
