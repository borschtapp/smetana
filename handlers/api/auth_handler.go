package api

import (
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/pkg/database"
	"borscht.app/smetana/pkg/errors"
	"borscht.app/smetana/pkg/services"
	"borscht.app/smetana/pkg/utils"
)

type AuthHandler struct {
	oidcService *services.OIDCService
}

func NewAuthHandler(oidcService *services.OIDCService) *AuthHandler {
	return &AuthHandler{
		oidcService: oidcService,
	}
}

type LoginForm struct {
	Email    string `validate:"required,email,min=6" json:"email"`
	Password string `validate:"required,min=8" json:"password"`
}

// Login godoc
// @Summary Authenticate user and return tokens.
// @Description Authenticate user by email and password, returning access and refresh tokens.
// @Tags auth
// @Accept json
// @Produce json
// @Param login body LoginForm true "Login credentials"
// @Success 200 {object} AuthResponse
// @Failure 400 {object} errors.Error
// @Failure 401 {object} errors.Error
// @Router /api/auth/login [post]
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var requestBody LoginForm
	if err := c.BodyParser(&requestBody); err != nil {
		return errors.BadRequest(err.Error())
	}

	var validate = validator.New()
	if err := validate.Struct(requestBody); err != nil {
		return errors.BadRequestVal(err)
	}

	var user domain.User
	if err := database.DB.Where(&domain.User{Email: requestBody.Email}).Find(&user).Error; err != nil {
		return err
	}

	if !utils.ValidatePassword(user.Password, requestBody.Password) {
		return errors.BadRequest("wrong user email address or password")
	}

	if tokens, err := h.generateTokens(user); err == nil {
		return c.JSON(tokens)
	} else {
		return err
	}
}

type AuthResponse struct {
	domain.User
	utils.Tokens
}

func (h *AuthHandler) generateTokens(user domain.User) (*AuthResponse, error) {
	tokens, err := utils.GenerateNewTokens(user.ID)
	if err != nil {
		return nil, err
	}

	// Set expires in for refresh key from .env file.
	expiresIn := time.Minute * time.Duration(utils.GetenvInt("JWT_REFRESH_EXPIRE_MINUTES", 10080))

	token := new(domain.UserToken)
	token.UserID = user.ID
	token.Type = "refresh"
	token.Token = tokens.Refresh
	token.Expires = time.Now().Add(expiresIn)

	if err := database.DB.Create(&token).Error; err != nil {
		return nil, err
	}

	return &AuthResponse{User: user, Tokens: *tokens}, nil
}

type RenewForm struct {
	RefreshToken string `validate:"required" json:"refresh_token"`
}

// Refresh godoc
// @Summary Refresh access token.
// @Description Refresh access token using a valid refresh token.
// @Tags auth
// @Accept json
// @Produce json
// @Param refresh body RenewForm true "Refresh token"
// @Success 200 {object} AuthResponse
// @Failure 400 {object} errors.Error
// @Failure 401 {object} errors.Error
// @Router /api/auth/refresh [post]
func (h *AuthHandler) Refresh(c *fiber.Ctx) error {
	var requestBody RenewForm
	if err := c.BodyParser(&requestBody); err != nil {
		return errors.BadRequest(err.Error())
	}

	var validate = validator.New()
	if err := validate.Struct(requestBody); err != nil {
		return errors.BadRequestVal(err)
	}

	var userToken domain.UserToken
	// Retrieve the token from database and check if it's valid.
	if err := database.DB.Joins("User").Where(&domain.UserToken{Token: requestBody.RefreshToken, Type: "refresh"}).First(&userToken).Error; err != nil {
		return errors.Unauthorized("Invalid refresh token")
	}

	// Checking, if now time greater than Refresh token expiration time.
	if time.Now().Before(userToken.Expires) {
		user := userToken.User
		if user == nil {
			return errors.Unauthorized("User not found for this token")
		}

		// remove old refresh token
		if err := database.DB.Unscoped().Where(&domain.UserToken{Token: userToken.Token}).Delete(&domain.UserToken{}).Error; err != nil {
			return err
		}

		if tokens, err := h.generateTokens(*user); err == nil {
			return c.JSON(tokens)
		} else {
			return err
		}
	} else {
		return errors.Unauthorized("unauthorized, your session was ended earlier")
	}
}

type RegisterForm struct {
	Name     string `validate:"min=2" json:"name"`
	Email    string `validate:"required,email,min=6" json:"email"`
	Password string `validate:"required,min=8" json:"password"`
}

// Register godoc
// @Summary Create a new user.
// @Description Register a new user with name, email, and password. Creates an associated personal Household.
// @Tags auth
// @Accept json
// @Produce json
// @Param user body RegisterForm true "User registration data"
// @Success 201 {object} AuthResponse
// @Failure 400 {object} errors.Error
// @Router /api/auth/register [post]
func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var requestBody RegisterForm
	if err := c.BodyParser(&requestBody); err != nil {
		return errors.BadRequest(err.Error())
	}

	var validate = validator.New()
	if err := validate.Struct(requestBody); err != nil {
		return errors.BadRequestVal(err)
	}

	var existingUser domain.User
	if err := database.DB.Where(&domain.User{Email: requestBody.Email}).First(&existingUser).Error; err == nil {
		return errors.BadRequest("user with this email already exists")
	}

	hash, err := utils.HashPassword(requestBody.Password)
	if err != nil {
		return errors.InternalServerError("Failed to hash password")
	}

	user := domain.User{
		ID:       uuid.New(),
		Email:    requestBody.Email,
		Password: hash,
		Created:  time.Now(),
	}
	if requestBody.Name != "" {
		user.Name = requestBody.Name
	} else {
		user.Name = strings.Split(requestBody.Email, "@")[0]
	}

	err = database.DB.Transaction(func(tx *gorm.DB) error {
		// 1. Create personal Household
		household := &domain.Household{
			Name: user.Name + "'s Household",
		}
		if err := tx.Create(household).Error; err != nil {
			return err
		}

		// 2. Create User linked to Household
		user.HouseholdID = household.ID
		user.Household = household
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "unique") {
			return errors.BadRequestField("Email", "already exists")
		}
		return err
	}

	if tokens, err := h.generateTokens(user); err == nil {
		return c.Status(fiber.StatusCreated).JSON(tokens)
	} else {
		return err
	}
}

// OIDCLogin godoc
// @Summary OIDC Initiator.
// @Description Redirects the user to the configured OIDC provider.
// @Tags auth
// @Success 302
// @Router /api/auth/oidc/login [get]
func (h *AuthHandler) OIDCLogin(c *fiber.Ctx) error {
	if h.oidcService == nil {
		return errors.NotImplemented("OIDC service not configured")
	}
	// Generate cryptographically secure random state for CSRF protection
	state := utils.GenerateRandomString(32)
	c.Cookie(&fiber.Cookie{
		Name:     "oidc_state",
		Value:    state,
		HTTPOnly: true,
		Secure:   true, // Ensure HTTPS in production
		SameSite: "Lax",
		MaxAge:   600, // 10 minutes
	})
	return c.Redirect(h.oidcService.GetLoginURL(state))
}

// OIDCCallback godoc
// @Summary OIDC Consumer.
// @Description Handles the callback from the identity provider and issues local tokens.
// @Tags auth
// @Success 200 {object} AuthResponse
// @Failure 400 {object} errors.Error
// @Failure 500 {object} errors.Error
// @Router /api/auth/oidc/callback [get]
func (h *AuthHandler) OIDCCallback(c *fiber.Ctx) error {
	if h.oidcService == nil {
		return errors.NotImplemented("OIDC service not configured")
	}

	// Validate state to prevent CSRF attacks
	state := c.Query("state")
	cookieState := c.Cookies("oidc_state")
	if state == "" || state != cookieState {
		return errors.BadRequest("Invalid or missing state parameter")
	}
	// Clear the state cookie
	c.ClearCookie("oidc_state")

	code := c.Query("code")
	if code == "" {
		return errors.BadRequest("Missing code")
	}

	_, idToken, err := h.oidcService.Exchange(c.Context(), code)
	if err != nil {
		return errors.BadRequest("Failed to exchange token: " + err.Error())
	}

	var claims struct {
		Email    string `json:"email"`
		Verified bool   `json:"email_verified"`
		Name     string `json:"name"`
		Sub      string `json:"sub"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return errors.InternalServerError("Failed to parse claims")
	}

	// Find or create user
	var user domain.User
	err = database.DB.Where(&domain.User{Email: claims.Email}).Preload("Household").First(&user).Error

	if err != nil && err == gorm.ErrRecordNotFound {
		// Create new user (JIT provisioning)
		user = domain.User{
			ID:      uuid.New(),
			Email:   claims.Email,
			Name:    claims.Name,
			Created: time.Now(),
			// No password for OIDC users
		}
		if user.Name == "" {
			user.Name = strings.Split(claims.Email, "@")[0]
		}

		// Create household
		household := &domain.Household{
			Name: user.Name + "'s Household",
		}
		if err := database.DB.Create(household).Error; err != nil {
			return err
		}
		user.Household = household
		user.HouseholdID = household.ID

		if err := database.DB.Create(&user).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	// Issue local tokens
	if tokens, err := h.generateTokens(user); err == nil {
		return c.JSON(tokens)
	} else {
		return err
	}
}
