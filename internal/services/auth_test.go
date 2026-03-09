package services_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/services"
	"borscht.app/smetana/internal/utils"
)

func newTestAuthService(userRepo *stubUserRepo) domain.AuthService {
	return services.NewAuthService(userRepo)
}

func TestAuthService_Login_ValidCredentials_ReturnsUser(t *testing.T) {
	hash, err := utils.HashPassword("correct-password")
	require.NoError(t, err)

	user := &domain.User{ID: uuid.New(), Email: "chef@borscht.app", Password: hash}
	repo := &stubUserRepo{
		byEmailFn: func(_ string) (*domain.User, error) { return user, nil },
	}

	svc := newTestAuthService(repo)
	got, err := svc.Login("chef@borscht.app", "correct-password")

	require.NoError(t, err)
	assert.Equal(t, user.ID, got.ID)
}

func TestAuthService_Login_WrongPassword_Unauthorized(t *testing.T) {
	hash, _ := utils.HashPassword("correct-password")
	user := &domain.User{ID: uuid.New(), Email: "chef@borscht.app", Password: hash}
	repo := &stubUserRepo{
		byEmailFn: func(_ string) (*domain.User, error) { return user, nil },
	}

	svc := newTestAuthService(repo)
	_, err := svc.Login("chef@borscht.app", "wrong-password")

	require.ErrorIs(t, err, sentinels.ErrUnauthorized)
}

func TestAuthService_Login_NonExistentEmail_Unauthorized(t *testing.T) {
	repo := &stubUserRepo{
		byEmailFn: func(_ string) (*domain.User, error) {
			return nil, sentinels.ErrRecordNotFound
		},
	}

	svc := newTestAuthService(repo)
	_, err := svc.Login("nobody@borscht.app", "any-password")

	require.ErrorIs(t, err, sentinels.ErrUnauthorized,
		"non-existent user must produce ErrUnauthorized, not ErrRecordNotFound")
}

func TestAuthService_Login_DBError_PropagatesError(t *testing.T) {
	dbErr := sentinels.ErrAlreadyExists // any non-NotFound error
	repo := &stubUserRepo{
		byEmailFn: func(_ string) (*domain.User, error) { return nil, dbErr },
	}

	svc := newTestAuthService(repo)
	_, err := svc.Login("chef@borscht.app", "password")

	require.ErrorIs(t, err, dbErr)
}

func TestAuthService_Register_CreatesUserWithHousehold(t *testing.T) {
	var created *domain.User
	repo := &stubUserRepo{
		createFn: func(u *domain.User) error {
			created = u
			return nil
		},
	}

	svc := newTestAuthService(repo)
	got, err := svc.Register("User", "chef@borscht.app", "password")

	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, created)
	assert.Equal(t, "User", created.Name)
	assert.Equal(t, "chef@borscht.app", created.Email)
	require.NotNil(t, created.Household, "household must be set so UserRepository can create it in the same transaction")
	assert.Contains(t, created.Household.Name, "User")
	assert.NotEqual(t, "password", created.Password, "password must be hashed")
	assert.True(t, utils.ValidatePassword(created.Password, "password"), "stored hash must validate against original password")
}

func TestAuthService_Register_EmptyName_UsesEmailPrefix(t *testing.T) {
	var created *domain.User
	repo := &stubUserRepo{
		createFn: func(u *domain.User) error { created = u; return nil },
	}

	svc := newTestAuthService(repo)
	_, err := svc.Register("", "second@borscht.app", "pass")

	require.NoError(t, err)
	assert.Equal(t, "second", created.Name, "empty name must fall back to the part before '@'")
}

func TestAuthService_Register_DuplicateEmail_ReturnsAlreadyExists(t *testing.T) {
	repo := &stubUserRepo{
		createFn: func(_ *domain.User) error { return sentinels.ErrAlreadyExists },
	}

	svc := newTestAuthService(repo)
	_, err := svc.Register("User", "chef@borscht.app", "pass")

	require.ErrorIs(t, err, sentinels.ErrAlreadyExists)
}

func TestAuthService_RotateRefreshToken_ValidToken_IssuesNewPair(t *testing.T) {
	// Set up a valid non-expired token that has a User preloaded on it.
	hid := uuid.New()
	user := &domain.User{ID: uuid.New(), HouseholdID: hid}
	tokenStr := "valid-refresh-token"
	userToken := &domain.UserToken{
		UserID:  user.ID,
		Type:    "refresh",
		Token:   tokenStr,
		Expires: time.Now().Add(time.Hour),
		User:    user,
	}

	var deletedToken string
	repo := &stubUserRepo{
		findTokenFn: func(tok, typ string) (*domain.UserToken, error) {
			assert.Equal(t, "refresh", typ)
			return userToken, nil
		},
		deleteTokenFn: func(tok string) error {
			deletedToken = tok
			return nil
		},
		createTokenFn: func(_ *domain.UserToken) error { return nil },
	}

	svc := newTestAuthService(repo)
	returnedUser, tokens, err := svc.RotateRefreshToken(tokenStr)

	require.NoError(t, err)
	require.NotNil(t, tokens)
	require.NotNil(t, returnedUser)
	assert.Equal(t, user.ID, returnedUser.ID)
	assert.NotEmpty(t, tokens.Access, "access token must be issued")
	assert.NotEmpty(t, tokens.Refresh, "refresh token must be issued")
	assert.Equal(t, tokenStr, deletedToken, "old refresh token must be invalidated")
}

func TestAuthService_RotateRefreshToken_ExpiredToken_Unauthorized(t *testing.T) {
	user := &domain.User{ID: uuid.New()}
	expiredToken := &domain.UserToken{
		Token:   "expired-token",
		Expires: time.Now().Add(-time.Hour), // already expired
		User:    user,
	}
	repo := &stubUserRepo{
		findTokenFn: func(_, _ string) (*domain.UserToken, error) { return expiredToken, nil },
	}

	svc := newTestAuthService(repo)
	_, _, err := svc.RotateRefreshToken("expired-token")

	require.ErrorIs(t, err, sentinels.ErrUnauthorized)
}

func TestAuthService_RotateRefreshToken_TokenNotFound_Unauthorized(t *testing.T) {
	repo := &stubUserRepo{
		findTokenFn: func(_, _ string) (*domain.UserToken, error) {
			return nil, sentinels.ErrRecordNotFound
		},
	}

	svc := newTestAuthService(repo)
	_, _, err := svc.RotateRefreshToken("ghost-token")

	require.ErrorIs(t, err, sentinels.ErrUnauthorized)
}

func TestAuthService_RotateRefreshToken_NilUser_Unauthorized(t *testing.T) {
	// A token row exists but its User association is nil (orphaned token).
	orphanToken := &domain.UserToken{
		Token:   "orphan",
		Expires: time.Now().Add(time.Hour),
		User:    nil, // no user loaded
	}
	repo := &stubUserRepo{
		findTokenFn: func(_, _ string) (*domain.UserToken, error) { return orphanToken, nil },
	}

	svc := newTestAuthService(repo)
	_, _, err := svc.RotateRefreshToken("orphan")

	require.ErrorIs(t, err, sentinels.ErrUnauthorized)
}
