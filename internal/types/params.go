package types

import (
	"borscht.app/smetana/internal/sentinels"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

func UuidParam(c fiber.Ctx, key string) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Params(key))
	if err != nil {
		return uuid.Nil, sentinels.BadRequest("malformed or missing param: " + key)
	}
	return id, nil
}

func UuidParams(c fiber.Ctx, first, second string) (uuid.UUID, uuid.UUID, error) {
	firstId, firstErr := uuid.Parse(c.Params(first))
	secondId, secondErr := uuid.Parse(c.Params(second))
	if firstErr != nil {
		return firstId, secondId, sentinels.BadRequest("malformed or missing param: " + first)
	}
	if secondErr != nil {
		return firstId, secondId, sentinels.BadRequest("malformed or missing param: " + second)
	}
	return firstId, secondId, nil
}
