package types

import (
	"fmt"
	"slices"
	"strings"

	"github.com/gofiber/fiber/v3"

	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/utils"
)

// PreloadOptions holds a list of relation names to eager-load.
type PreloadOptions struct {
	Preload []string
}

// Has checks if a specific `relation` is requested, expects the `relation` to be lower case.
func (p PreloadOptions) Has(relation string) bool {
	return slices.Contains(p.Preload, relation)
}

// Validate returns a 400 error if any requested preload value is not in the allowed list.
func (p PreloadOptions) Validate(allowed ...string) error {
	for _, rel := range p.Preload {
		if !slices.Contains(allowed, rel) {
			return sentinels.BadRequest(fmt.Sprintf("invalid preload option '%s', allowed: %s", rel, strings.Join(allowed, ", ")))
		}
	}
	return nil
}

// GetPreloadOptions parses the "preload" query parameter from the request.
func GetPreloadOptions(c fiber.Ctx) PreloadOptions {
	return PreloadOptions{Preload: utils.CsvSplit(c.Query("preload"))} // returns lowercased strings in a slice
}
