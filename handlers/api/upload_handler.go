package api

import (
	"io"
	"net/http"
	"path/filepath"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"borscht.app/smetana/pkg/errors"
	"borscht.app/smetana/pkg/services"
)

type UploadHandler struct {
	imageService *services.ImageService
}

func NewUploadHandler(imageService *services.ImageService) *UploadHandler {
	return &UploadHandler{
		imageService: imageService,
	}
}

// Upload godoc
// @Summary Upload an image.
// @Description Upload an image file via multipart form. Returns the public URL.
// @Tags infrastructure
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "Image file"
// @Success 201 {object} services.UploadedImage
// @Failure 400 {object} errors.Error
// @Failure 401 {object} errors.Error
// @Security ApiKeyAuth
// @Router /api/uploads [post]
func (h *UploadHandler) Upload(c fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return errors.BadRequest("Missing file: " + err.Error())
	}

	src, err := file.Open()
	if err != nil {
		return errors.BadRequest("Failed to open file")
	}
	defer src.Close()

	data, err := io.ReadAll(src)
	if err != nil {
		return errors.BadRequest("Failed to read file")
	}

	// Detect content type
	contentType := file.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	// Generate random filename
	ext := filepath.Ext(file.Filename)
	filename := uuid.New().String() + ext
	path := "uploads/" + filename

	// Save
	uploaded, err := h.imageService.SaveImage(path, data, contentType)
	if err != nil {
		return errors.InternalServerError("Failed to save image: " + err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(uploaded)
}
