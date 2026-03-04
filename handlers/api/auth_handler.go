package api

import (
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
	"github.com/google/uuid"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/pkg/sentinels"
	"borscht.app/smetana/pkg/utils"
)

var validate = validator.New()

type AuthHandler struct {
	oidcService domain.OIDCService
	userService domain.UserService
}

func NewAuthHandler(oidcService domain.OIDCService, userService domain.UserService) *AuthHandler {
	return &AuthHandler{
		oidcService: oidcService,
		userService: userService,
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
// @Failure 400 {object} domain.Error
// @Failure 401 {object} domain.Error
// @Router /api/v1/auth/login [post]
func (h *AuthHandler) Login(c fiber.Ctx) error {
	var requestBody LoginForm
	if err := c.Bind().Body(&requestBody); err != nil {
		return sentinels.BadRequest(err.Error())
	}

	if err := validate.Struct(requestBody); err != nil {
		return sentinels.BadRequestVal(err)
	}

	user, err := h.userService.ByEmail(requestBody.Email)
	if err != nil {
		return err
	}

	if !utils.ValidatePassword(user.Password, requestBody.Password) {
		return sentinels.BadRequest("wrong user email address or password")
	}

	if tokens, err := h.issueTokens(*user); err == nil {
		return c.JSON(tokens)
	} else {
		return err
	}
}

type AuthResponse struct {
	domain.User
	utils.Tokens
}

func (h *AuthHandler) issueTokens(user domain.User) (*AuthResponse, error) {
	tokens, err := utils.GenerateNewTokens(user.ID, user.HouseholdID)
	if err != nil {
		return nil, err
	}

	expiresIn := time.Minute * time.Duration(utils.GetenvInt("JWT_REFRESH_EXPIRE_MINUTES", 10080))

	token := &domain.UserToken{
		UserID:  user.ID,
		Type:    "refresh",
		Token:   tokens.Refresh,
		Expires: time.Now().Add(expiresIn),
	}

	if err := h.userService.CreateRefreshToken(token); err != nil {
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
// @Failure 400 {object} domain.Error
// @Failure 401 {object} domain.Error
// @Router /api/v1/auth/refresh [post]
func (h *AuthHandler) Refresh(c fiber.Ctx) error {
	var requestBody RenewForm
	if err := c.Bind().Body(&requestBody); err != nil {
		return sentinels.BadRequest(err.Error())
	}

	if err := validate.Struct(requestBody); err != nil {
		return sentinels.BadRequestVal(err)
	}

	userToken, err := h.userService.FindRefreshToken(requestBody.RefreshToken)
	if err != nil {
		return sentinels.Unauthorized("Invalid refresh token")
	}

	if time.Now().Before(userToken.Expires) {
		user := userToken.User
		if user == nil {
			return sentinels.Unauthorized("User not found for this token")
		}

		if err := h.userService.DeleteRefreshToken(userToken.Token); err != nil {
			return err
		}

		if tokens, err := h.issueTokens(*user); err == nil {
			return c.JSON(tokens)
		} else {
			return err
		}
	} else {
		return sentinels.Unauthorized("unauthorized, your session was ended earlier")
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
// @Failure 400 {object} domain.Error
// @Router /api/v1/auth/register [post]
func (h *AuthHandler) Register(c fiber.Ctx) error {
	var requestBody RegisterForm
	if err := c.Bind().Body(&requestBody); err != nil {
		return sentinels.BadRequest(err.Error())
	}

	if err := validate.Struct(requestBody); err != nil {
		return sentinels.BadRequestVal(err)
	}

	if _, err := h.userService.ByEmail(requestBody.Email); err == nil {
		return sentinels.BadRequest("user with this email already exists")
	}

	hash, err := utils.HashPassword(requestBody.Password)
	if err != nil {
		return sentinels.InternalServerError("Failed to hash password")
	}

	name := requestBody.Name
	if name == "" {
		name = strings.Split(requestBody.Email, "@")[0]
	}

	user := domain.User{
		ID:       uuid.New(),
		Email:    requestBody.Email,
		Password: hash,
		Name:     name,
		Created:  time.Now(),
	}

	if err := h.userService.Create(&user); err != nil {
		return err
	}

	if tokens, err := h.issueTokens(user); err == nil {
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
// @Router /api/v1/auth/oidc/login [get]
func (h *AuthHandler) OIDCLogin(c fiber.Ctx) error {
	if h.oidcService == nil {
		return sentinels.NotImplemented("OIDC service not configured")
	}

	state := utils.GenerateRandomString(32)
	c.Cookie(&fiber.Cookie{
		Name:     "oidc_state",
		Value:    state,
		HTTPOnly: true,
		Secure:   true, // Ensure HTTPS in production
		SameSite: "Lax",
		MaxAge:   600, // 10 minutes
	})
	return c.Redirect().To(h.oidcService.LoginURL(state))
}

// OIDCCallback godoc
// @Summary OIDC Consumer.
// @Description Handles the callback from the identity provider and issues local tokens.
// @Tags auth
// @Success 200 {object} AuthResponse
// @Failure 400 {object} domain.Error
// @Failure 500 {object} domain.Error
// @Param state query string true "CSRF State"
// @Param code query string true "Auth Code"
// @Router /api/v1/auth/oidc/callback [get]
func (h *AuthHandler) OIDCCallback(c fiber.Ctx) error {
	if h.oidcService == nil {
		return sentinels.NotImplemented("OIDC service not configured")
	}

	state := c.Query("state")
	cookieState := c.Cookies("oidc_state")
	if state == "" || state != cookieState {
		return sentinels.BadRequest("Invalid or missing state parameter")
	}

	c.ClearCookie("oidc_state")

	code := c.Query("code")
	if code == "" {
		return sentinels.BadRequest("Missing code")
	}

	_, idToken, err := h.oidcService.Exchange(c.RequestCtx(), code)
	if err != nil {
		log.Warnf("OIDC token exchange failed: %v", err)
		return sentinels.BadRequest("Failed to exchange token")
	}

	var claims struct {
		Email    string `json:"email"`
		Verified bool   `json:"email_verified"`
		Name     string `json:"name"`
		Sub      string `json:"sub"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return sentinels.InternalServerError("Failed to parse claims")
	}

	user, err := h.oidcService.FindOrRegisterOIDCUser(claims.Email, claims.Name)
	if err != nil {
		return err
	}

	if tokens, err := h.issueTokens(*user); err == nil {
		return c.JSON(tokens)
	} else {
		return err
	}
}
