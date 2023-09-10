package handlers

import "github.com/gofiber/fiber/v2"

type SuccessMessage struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// HealthCheck godoc
// @Summary Show the status of server.
// @Description get the status of server.
// @Tags root
// @Accept */*
// @Produce json
// @Success 200 {object} SuccessMessage
// @Router /_health [get]
func HealthCheck(c *fiber.Ctx) error {
	return c.JSON(SuccessMessage{
		Success: true,
		Message: "Online, caffeinated, and ready to rock your requests!",
	})
}
