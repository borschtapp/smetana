package api

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/utils"
)

type AuthHandler struct {
	authService domain.AuthService
	oidcService domain.OIDCService
}

func NewAuthHandler(authService domain.AuthService, oidcService domain.OIDCService) *AuthHandler {
	return &AuthHandler{authService: authService, oidcService: oidcService}
}

type AuthResponse struct {
	domain.User
	domain.AuthTokens
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
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Router /api/v1/auth/login [post]
func (h *AuthHandler) Login(c fiber.Ctx) error {
	var body LoginForm
	if err := c.Bind().Body(&body); err != nil {
		return sentinels.BadRequest(err.Error())
	}
	if err := validate.Struct(body); err != nil {
		return sentinels.BadRequestVal(err)
	}

	user, err := h.authService.Login(body.Email, body.Password)
	if err != nil {
		return err
	}
	tokens, err := h.authService.IssueTokens(*user)
	if err != nil {
		return err
	}
	return c.JSON(AuthResponse{User: *user, AuthTokens: *tokens})
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
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Router /api/v1/auth/refresh [post]
func (h *AuthHandler) Refresh(c fiber.Ctx) error {
	var body RenewForm
	if err := c.Bind().Body(&body); err != nil {
		return sentinels.BadRequest(err.Error())
	}
	if err := validate.Struct(body); err != nil {
		return sentinels.BadRequestVal(err)
	}

	user, tokens, err := h.authService.RotateRefreshToken(body.RefreshToken)
	if err != nil {
		return err
	}
	return c.JSON(AuthResponse{User: *user, AuthTokens: *tokens})
}

// Logout godoc
// @Summary Logout user.
// @Description Invalidate a refresh token, ending the session.
// @Tags auth
// @Accept json
// @Produce json
// @Param logout body RenewForm true "Refresh token to invalidate"
// @Success 204
// @Failure 400 {object} sentinels.Error
// @Router /api/v1/auth/logout [post]
func (h *AuthHandler) Logout(c fiber.Ctx) error {
	var body RenewForm
	if err := c.Bind().Body(&body); err != nil {
		return sentinels.BadRequest(err.Error())
	}
	if err := validate.Struct(body); err != nil {
		return sentinels.BadRequestVal(err)
	}
	if err := h.authService.Logout(body.RefreshToken); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
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
// @Failure 400 {object} sentinels.Error
// @Failure 409 {object} sentinels.Error
// @Router /api/v1/auth/register [post]
func (h *AuthHandler) Register(c fiber.Ctx) error {
	var body RegisterForm
	if err := c.Bind().Body(&body); err != nil {
		return sentinels.BadRequest(err.Error())
	}
	if err := validate.Struct(body); err != nil {
		return sentinels.BadRequestVal(err)
	}

	user, err := h.authService.Register(body.Name, body.Email, body.Password)
	if err != nil {
		return err
	}
	tokens, err := h.authService.IssueTokens(*user)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(AuthResponse{User: *user, AuthTokens: *tokens})
}

// OIDCLogin godoc
// @Summary OIDC Initiator.
// @Description Redirects the user to the configured OIDC provider.
// @Tags auth
// @Success 302
// @Router /api/v1/auth/oidc/login [get]
func (h *AuthHandler) OIDCLogin(c fiber.Ctx) error {
	if h.oidcService == nil {
		return sentinels.NotImplemented("OIDC services not configured")
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
// @Failure 400 {object} sentinels.Error
// @Failure 500 {object} sentinels.Error
// @Param state query string true "CSRF State"
// @Param code query string true "Auth Code"
// @Router /api/v1/auth/oidc/callback [get]
func (h *AuthHandler) OIDCCallback(c fiber.Ctx) error {
	if h.oidcService == nil {
		return sentinels.NotImplemented("OIDC services not configured")
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
		log.Warnw("OIDC token exchange failed", "error", err)
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
	if !claims.Verified {
		return sentinels.BadRequest("Email address not verified by identity provider")
	}

	user, err := h.oidcService.Authorize(claims.Email, claims.Name)
	if err != nil {
		return err
	}
	tokens, err := h.authService.IssueTokens(*user)
	if err != nil {
		return err
	}
	return c.JSON(AuthResponse{User: *user, AuthTokens: *tokens})
}
