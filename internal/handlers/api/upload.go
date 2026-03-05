package api

import (
	"io"
	"net/http"
	"path/filepath"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
)

type UploadHandler struct {
	imageService domain.ImageService
}

func NewUploadHandler(imageService domain.ImageService) *UploadHandler {
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
// @Success 201 {object} domain.UploadedImage
// @Failure 400 {object} domain.Error
// @Failure 401 {object} domain.Error
// @Security ApiKeyAuth
// @Router /api/v1/uploads [post]
func (h *UploadHandler) Upload(c fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return sentinels.BadRequest("Missing file: " + err.Error())
	}

	src, err := file.Open()
	if err != nil {
		return sentinels.BadRequest("Failed to open file")
	}
	defer src.Close()

	data, err := io.ReadAll(src)
	if err != nil {
		return sentinels.BadRequest("Failed to read file")
	}

	// Detect content type
	contentType := file.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	// Generate random filename
	filenameUuid, err := uuid.NewV7()
	if err != nil {
		return err
	}

	ext := filepath.Ext(file.Filename)
	path := "uploads/" + filenameUuid.String() + ext

	uploaded, err := h.imageService.SaveImageData(path, data, contentType)
	if err != nil {
		return sentinels.InternalServerError("Failed to save image: " + err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(uploaded)
}
