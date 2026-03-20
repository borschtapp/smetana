package services_test

import (
	"crypto/sha256"
	"encoding/hex"
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

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func newTestAuthService(userRepo *stubUserRepo) domain.AuthService {
	return services.NewAuthService(userRepo, nil)
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
			return nil, sentinels.ErrNotFound
		},
	}

	svc := newTestAuthService(repo)
	_, err := svc.Login("nobody@borscht.app", "any-password")

	require.ErrorIs(t, err, sentinels.ErrUnauthorized,
		"non-existent user must produce ErrUnauthorized, not ErrNotFound")
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
	got, err := svc.Register("User", "chef@borscht.app", "password", "")

	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, created)
	assert.Equal(t, "User", created.Name)
	assert.Equal(t, "chef@borscht.app", created.Email)
	assert.Equal(t, uuid.Nil, created.HouseholdID, "service passes nil HouseholdID; repository will create household")
	assert.NotEqual(t, "password", created.Password, "password must be hashed")
	assert.True(t, utils.ValidatePassword(created.Password, "password"), "stored hash must validate against original password")
}

func TestAuthService_Register_EmptyName_UsesEmailPrefix(t *testing.T) {
	var created *domain.User
	repo := &stubUserRepo{
		createFn: func(u *domain.User) error { created = u; return nil },
	}

	svc := newTestAuthService(repo)
	_, err := svc.Register("", "second@borscht.app", "pass", "")

	require.NoError(t, err)
	assert.Equal(t, "second", created.Name, "empty name must fall back to the part before '@'")
}

func TestAuthService_Register_DuplicateEmail_ReturnsAlreadyExists(t *testing.T) {
	repo := &stubUserRepo{
		createFn: func(_ *domain.User) error { return sentinels.ErrAlreadyExists },
	}

	svc := newTestAuthService(repo)
	_, err := svc.Register("User", "chef@borscht.app", "pass", "")

	require.ErrorIs(t, err, sentinels.ErrAlreadyExists)
}

func TestAuthService_RotateRefreshToken_ValidToken_IssuesNewPair(t *testing.T) {
	// Set up a valid non-expired token that has a User preloaded on it.
	hid := uuid.New()
	user := &domain.User{ID: uuid.New(), HouseholdID: hid}
	rawToken := "valid-refresh-token"
	hashedToken := hashToken(rawToken)
	userToken := &domain.UserToken{
		UserID:  user.ID,
		Type:    domain.TokenTypeRefresh,
		Token:   hashedToken, // DB stores the hash, not the raw value
		Expires: time.Now().Add(time.Hour),
		User:    user,
	}

	var deletedToken string
	repo := &stubUserRepo{
		findTokenFn: func(tok, typ string) (*domain.UserToken, error) {
			assert.Equal(t, hashedToken, tok, "service must hash the token before lookup")
			assert.Equal(t, domain.TokenTypeRefresh, typ)
			return userToken, nil
		},
		deleteTokenFn: func(tok string) (bool, error) {
			deletedToken = tok
			return true, nil
		},
		createTokenFn: func(_ *domain.UserToken) error { return nil },
	}

	svc := newTestAuthService(repo)
	returnedUser, tokens, err := svc.RotateRefreshToken(rawToken)

	require.NoError(t, err)
	require.NotNil(t, tokens)
	require.NotNil(t, returnedUser)
	assert.Equal(t, user.ID, returnedUser.ID)
	assert.NotEmpty(t, tokens.Access, "access token must be issued")
	assert.NotEmpty(t, tokens.Refresh, "refresh token must be issued")
	assert.Equal(t, hashedToken, deletedToken, "old token hash must be invalidated")
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
			return nil, sentinels.ErrNotFound
		},
	}

	svc := newTestAuthService(repo)
	_, _, err := svc.RotateRefreshToken("ghost-token")

	require.ErrorIs(t, err, sentinels.ErrUnauthorized)
}

func TestAuthService_RotateRefreshToken_NilUser_Unauthorized(t *testing.T) {
	// A token row exists but its User association is nil (orphaned token).
	orphanToken := &domain.UserToken{
		Token:   hashToken("orphan"),
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

func TestAuthService_RotateRefreshToken_TokenAlreadyDeleted_Unauthorized(t *testing.T) {
	hid := uuid.New()
	user := &domain.User{ID: uuid.New(), HouseholdID: hid}
	userToken := &domain.UserToken{
		UserID:  user.ID,
		Type:    domain.TokenTypeRefresh,
		Token:   hashToken("race-token"),
		Expires: time.Now().Add(time.Hour),
		User:    user,
	}
	repo := &stubUserRepo{
		findTokenFn: func(_, _ string) (*domain.UserToken, error) { return userToken, nil },
		deleteTokenFn: func(_ string) (bool, error) {
			// Simulate second concurrent request: token was already deleted
			return false, nil
		},
	}

	svc := newTestAuthService(repo)
	_, _, err := svc.RotateRefreshToken("race-token")

	require.ErrorIs(t, err, sentinels.ErrUnauthorized,
		"must reject rotation when token already deleted (race condition protection)")
}

func TestAuthService_Logout_DeletesHashedToken(t *testing.T) {
	rawToken := "logout-token"
	var deletedToken string
	repo := &stubUserRepo{
		deleteTokenFn: func(tok string) (bool, error) {
			deletedToken = tok
			return true, nil
		},
	}

	svc := newTestAuthService(repo)
	err := svc.Logout(rawToken)

	require.NoError(t, err)
	assert.Equal(t, hashToken(rawToken), deletedToken, "logout must delete the hash, not the raw token")
}

func TestAuthService_Logout_PropagatesDeleteError(t *testing.T) {
	repo := &stubUserRepo{
		deleteTokenFn: func(_ string) (bool, error) { return false, sentinels.ErrNotFound },
	}

	svc := newTestAuthService(repo)
	err := svc.Logout("any-token")

	require.ErrorIs(t, err, sentinels.ErrNotFound)
}

func TestAuthService_Register_WithInviteCode_ValidToken_JoinsHousehold(t *testing.T) {
	inviterID := uuid.New()
	householdID := uuid.New()
	inviter := &domain.User{ID: inviterID, HouseholdID: householdID}
	inviteToken := &domain.UserToken{
		Token:   "VALIDCODE",
		Type:    domain.TokenTypeHouseholdInvite,
		Expires: time.Now().Add(time.Hour),
		User:    inviter,
	}

	var created *domain.User
	var deletedToken string
	repo := &stubUserRepo{
		findTokenFn: func(tok, typ string) (*domain.UserToken, error) {
			assert.Equal(t, "VALIDCODE", tok)
			assert.Equal(t, domain.TokenTypeHouseholdInvite, typ)
			return inviteToken, nil
		},
		createFn: func(u *domain.User) error {
			created = u
			return nil
		},
		deleteTokenFn: func(tok string) (bool, error) {
			deletedToken = tok
			return true, nil
		},
	}

	svc := newTestAuthService(repo)
	user, err := svc.Register("NewMember", "newmember@borscht.app", "password", "VALIDCODE")

	require.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, householdID, created.HouseholdID, "user must join the inviter's household")
	assert.Equal(t, "VALIDCODE", deletedToken, "invite code must be consumed")
}

func TestAuthService_Register_WithInviteCode_NilUser_BadRequest(t *testing.T) {
	// Orphaned invite token: token row exists but its User association is nil
	orphanToken := &domain.UserToken{
		Token:   "ORPHANCODE",
		Type:    domain.TokenTypeHouseholdInvite,
		Expires: time.Now().Add(time.Hour),
		User:    nil, // no user loaded — the bug scenario
	}

	repo := &stubUserRepo{
		findTokenFn: func(_, _ string) (*domain.UserToken, error) { return orphanToken, nil },
	}

	svc := newTestAuthService(repo)
	_, err := svc.Register("Hacker", "hacker@borscht.app", "password", "ORPHANCODE")

	var sentinelErr *sentinels.Error
	require.ErrorAs(t, err, &sentinelErr, "nil user in token must return a sentinel error, not panic")
	assert.Equal(t, 400, sentinelErr.Status, "must return BadRequest (400)")
}

func TestAuthService_Register_WithInviteCode_ExpiredToken_BadRequest(t *testing.T) {
	expiredToken := &domain.UserToken{
		Token:   "EXPIREDCODE",
		Type:    domain.TokenTypeHouseholdInvite,
		Expires: time.Now().Add(-time.Hour), // already expired
		User:    &domain.User{ID: uuid.New()},
	}

	repo := &stubUserRepo{
		findTokenFn: func(_, _ string) (*domain.UserToken, error) { return expiredToken, nil },
	}

	svc := newTestAuthService(repo)
	_, err := svc.Register("Guest", "guest@borscht.app", "password", "EXPIREDCODE")

	var sentinelErr *sentinels.Error
	require.ErrorAs(t, err, &sentinelErr)
	assert.Equal(t, 400, sentinelErr.Status)
}
