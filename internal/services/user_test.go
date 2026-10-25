package services_test

import (
	"errors"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/services"
	"borscht.app/smetana/internal/utils"
)

var errDB = errors.New("db error")

func newTestUserService(userRepo *stubUserRepo, householdRepo domain.HouseholdRepository) domain.UserService {
	return services.NewUserService(userRepo, householdRepo)
}

func TestUserService_ByID_DifferentRequester_Forbidden(t *testing.T) {
	svc := newTestUserService(&stubUserRepo{}, &stubHouseholdRepo{})
	_, err := svc.ByID(uuid.New(), uuid.New())
	require.ErrorIs(t, err, sentinels.ErrForbidden)
}

func TestUserService_ByID_Success(t *testing.T) {
	userID := uuid.New()
	expected := &domain.User{ID: userID, Name: "Test User"}
	userRepo := &stubUserRepo{
		byIDFn: func(id uuid.UUID) (*domain.User, error) {
			if id == userID {
				return expected, nil
			}
			return nil, sentinels.ErrNotFound
		},
	}
	svc := newTestUserService(userRepo, &stubHouseholdRepo{})
	user, err := svc.ByID(userID, userID)
	require.NoError(t, err)
	assert.Equal(t, expected, user)
}

func TestUserService_ByID_RepoError(t *testing.T) {
	userID := uuid.New()
	userRepo := &stubUserRepo{
		byIDFn: func(_ uuid.UUID) (*domain.User, error) {
			return nil, errDB
		},
	}
	svc := newTestUserService(userRepo, &stubHouseholdRepo{})
	_, err := svc.ByID(userID, userID)
	require.ErrorIs(t, err, errDB)
}

func TestUserService_Update_DifferentRequester_Forbidden(t *testing.T) {
	svc := newTestUserService(&stubUserRepo{}, &stubHouseholdRepo{})
	_, err := svc.Update(uuid.New(), uuid.New(), new("New Name"), nil, nil, nil, nil)
	require.ErrorIs(t, err, sentinels.ErrForbidden)
}

func TestUserService_Update_PasswordChange_RevokesOtherRefreshTokens(t *testing.T) {
	userID := uuid.New()
	hash, err := utils.HashPassword("current-password")
	require.NoError(t, err)
	user := &domain.User{ID: userID, Password: hash}

	var revokedUserID uuid.UUID
	var revokedType string
	var exempted *string
	userRepo := &stubUserRepo{
		byIDFn:   func(_ uuid.UUID) (*domain.User, error) { return user, nil },
		updateFn: func(_ *domain.User) error { return nil },
		deleteTokensByUserExceptFn: func(id uuid.UUID, tokenType string, exceptTokenHash *string) error {
			revokedUserID = id
			revokedType = tokenType
			exempted = exceptTokenHash
			return nil
		},
	}

	svc := newTestUserService(userRepo, &stubHouseholdRepo{})
	_, err = svc.Update(userID, userID, nil, nil, new("current-password"), new("new-password"), nil)

	require.NoError(t, err)
	assert.Equal(t, userID, revokedUserID)
	assert.Equal(t, domain.TokenTypeRefresh, revokedType, "changing the password must revoke refresh tokens")
	assert.Nil(t, exempted, "no current refresh token supplied means every session is revoked")
}

func TestUserService_Update_PasswordChange_WithCurrentRefreshToken_PreservesCallerSession(t *testing.T) {
	userID := uuid.New()
	hash, err := utils.HashPassword("current-password")
	require.NoError(t, err)
	user := &domain.User{ID: userID, Password: hash}

	var exempted *string
	userRepo := &stubUserRepo{
		byIDFn:   func(_ uuid.UUID) (*domain.User, error) { return user, nil },
		updateFn: func(_ *domain.User) error { return nil },
		deleteTokensByUserExceptFn: func(_ uuid.UUID, _ string, exceptTokenHash *string) error {
			exempted = exceptTokenHash
			return nil
		},
	}

	tokenHash := utils.HashToken("my-refresh-token")
	svc := newTestUserService(userRepo, &stubHouseholdRepo{})
	_, err = svc.Update(userID, userID, nil, nil, new("current-password"), new("new-password"), &tokenHash)

	require.NoError(t, err)
	require.NotNil(t, exempted)
	assert.Equal(t, tokenHash, *exempted,
		"the caller's own refresh token hash must be exempted so their session survives the password change, unchanged from what the caller supplied")
}

func TestUserService_Update_EmailChange_RevokesOtherRefreshTokens(t *testing.T) {
	userID := uuid.New()
	hash, err := utils.HashPassword("current-password")
	require.NoError(t, err)
	user := &domain.User{ID: userID, Password: hash}

	revoked := false
	userRepo := &stubUserRepo{
		byIDFn:   func(_ uuid.UUID) (*domain.User, error) { return user, nil },
		updateFn: func(_ *domain.User) error { return nil },
		deleteTokensByUserExceptFn: func(_ uuid.UUID, tokenType string, _ *string) error {
			if tokenType == domain.TokenTypeRefresh {
				revoked = true
			}
			return nil
		},
	}

	svc := newTestUserService(userRepo, &stubHouseholdRepo{})
	_, err = svc.Update(userID, userID, nil, new("new@borscht.app"), new("current-password"), nil, nil)

	require.NoError(t, err)
	assert.True(t, revoked, "changing the email must revoke refresh tokens")
}

func TestUserService_Update_NameOnly_DoesNotRevokeTokens(t *testing.T) {
	userID := uuid.New()
	user := &domain.User{ID: userID, Password: "irrelevant-hash"}

	revokeCalled := false
	userRepo := &stubUserRepo{
		byIDFn:   func(_ uuid.UUID) (*domain.User, error) { return user, nil },
		updateFn: func(_ *domain.User) error { return nil },
		deleteTokensByUserExceptFn: func(_ uuid.UUID, _ string, _ *string) error {
			revokeCalled = true
			return nil
		},
	}

	svc := newTestUserService(userRepo, &stubHouseholdRepo{})
	_, err := svc.Update(userID, userID, new("New Name"), nil, nil, nil, nil)

	require.NoError(t, err)
	assert.False(t, revokeCalled, "a name-only update must not require a current password or revoke sessions")
}

func TestUserService_Update_WrongCurrentPassword_Unprocessable(t *testing.T) {
	userID := uuid.New()
	hash, err := utils.HashPassword("correct-password")
	require.NoError(t, err)
	user := &domain.User{ID: userID, Password: hash}

	userRepo := &stubUserRepo{
		byIDFn: func(_ uuid.UUID) (*domain.User, error) { return user, nil },
	}

	svc := newTestUserService(userRepo, &stubHouseholdRepo{})
	_, err = svc.Update(userID, userID, nil, nil, new("wrong-password"), new("new-password"), nil)

	var se *sentinels.Error
	require.ErrorAs(t, err, &se)
	assert.Equal(t, fiber.StatusUnprocessableEntity, se.Status, "a wrong password must not look like an expired session (401)")
	assert.Contains(t, se.Fields, "current_password")
}

func TestUserService_Delete_DifferentRequester_Forbidden(t *testing.T) {
	deleteCalled := false
	userRepo := &stubUserRepo{
		deleteFn: func(_ uuid.UUID) error { deleteCalled = true; return nil },
	}
	svc := newTestUserService(userRepo, &stubHouseholdRepo{})

	err := svc.Delete(uuid.New(), uuid.New())

	require.ErrorIs(t, err, sentinels.ErrForbidden)
	assert.False(t, deleteCalled, "must not touch the repository when the requester doesn't own the account")
}

func TestUserService_Delete_UserNotFound_ReturnsError(t *testing.T) {
	userID := uuid.New()
	userRepo := &stubUserRepo{
		byIDFn: func(_ uuid.UUID) (*domain.User, error) {
			return nil, sentinels.ErrNotFound
		},
	}
	svc := newTestUserService(userRepo, &stubHouseholdRepo{})

	err := svc.Delete(userID, userID)

	require.ErrorIs(t, err, sentinels.ErrNotFound)
}

func TestUserService_Delete_UserRepoFails_ReturnsError(t *testing.T) {
	userID := uuid.New()
	userRepo := &stubUserRepo{
		byIDFn: func(_ uuid.UUID) (*domain.User, error) {
			return &domain.User{ID: userID, HouseholdID: uuid.New()}, nil
		},
		deleteFn: func(_ uuid.UUID) error {
			return errDB
		},
	}
	svc := newTestUserService(userRepo, &stubHouseholdRepo{})

	err := svc.Delete(userID, userID)

	require.ErrorIs(t, err, errDB)
}

func TestUserService_Delete_HouseholdHasOtherMembers_DoesNotDeleteHousehold(t *testing.T) {
	userID := uuid.New()
	householdID := uuid.New()
	var deletedUserID uuid.UUID
	householdDeleteCalled := false

	userRepo := &stubUserRepo{
		byIDFn: func(_ uuid.UUID) (*domain.User, error) {
			return &domain.User{ID: userID, HouseholdID: householdID}, nil
		},
		deleteFn: func(id uuid.UUID) error {
			deletedUserID = id
			return nil
		},
	}
	householdRepo := &stubHouseholdRepo{
		membersFn: func(hID uuid.UUID, _, _ int) ([]domain.User, int64, error) {
			assert.Equal(t, householdID, hID)
			return []domain.User{{ID: uuid.New()}}, 1, nil
		},
		deleteFn: func(_ uuid.UUID) error {
			householdDeleteCalled = true
			return nil
		},
	}

	svc := newTestUserService(userRepo, householdRepo)
	err := svc.Delete(userID, userID)

	require.NoError(t, err)
	assert.Equal(t, userID, deletedUserID)
	assert.False(t, householdDeleteCalled, "household must not be deleted if members remain")
}

func TestUserService_Delete_HouseholdHasNoOtherMembers_DeletesHousehold(t *testing.T) {
	userID := uuid.New()
	householdID := uuid.New()
	var deletedUserID uuid.UUID
	var deletedHouseholdID uuid.UUID

	userRepo := &stubUserRepo{
		byIDFn: func(_ uuid.UUID) (*domain.User, error) {
			return &domain.User{ID: userID, HouseholdID: householdID}, nil
		},
		deleteFn: func(id uuid.UUID) error {
			deletedUserID = id
			return nil
		},
	}
	householdRepo := &stubHouseholdRepo{
		membersFn: func(hID uuid.UUID, _, _ int) ([]domain.User, int64, error) {
			assert.Equal(t, householdID, hID)
			return nil, 0, nil
		},
		deleteFn: func(hID uuid.UUID) error {
			deletedHouseholdID = hID
			return nil
		},
	}

	svc := newTestUserService(userRepo, householdRepo)
	err := svc.Delete(userID, userID)

	require.NoError(t, err)
	assert.Equal(t, userID, deletedUserID)
	assert.Equal(t, householdID, deletedHouseholdID)
}

func TestUserService_Delete_MembersCheckFails_ReturnsError(t *testing.T) {
	userID := uuid.New()
	userRepo := &stubUserRepo{
		byIDFn: func(_ uuid.UUID) (*domain.User, error) {
			return &domain.User{ID: userID, HouseholdID: uuid.New()}, nil
		},
		deleteFn: func(_ uuid.UUID) error { return nil },
	}
	householdRepo := &stubHouseholdRepo{
		membersFn: func(_ uuid.UUID, _, _ int) ([]domain.User, int64, error) {
			return nil, 0, errDB
		},
	}

	svc := newTestUserService(userRepo, householdRepo)
	err := svc.Delete(userID, userID)

	require.ErrorIs(t, err, errDB)
}

func TestUserService_Delete_HouseholdDeleteFails_ReturnsError(t *testing.T) {
	userID := uuid.New()
	userRepo := &stubUserRepo{
		byIDFn: func(_ uuid.UUID) (*domain.User, error) {
			return &domain.User{ID: userID, HouseholdID: uuid.New()}, nil
		},
		deleteFn: func(_ uuid.UUID) error { return nil },
	}
	householdRepo := &stubHouseholdRepo{
		membersFn: func(_ uuid.UUID, _, _ int) ([]domain.User, int64, error) {
			return nil, 0, nil
		},
		deleteFn: func(_ uuid.UUID) error {
			return errDB
		},
	}

	svc := newTestUserService(userRepo, householdRepo)
	err := svc.Delete(userID, userID)

	require.ErrorIs(t, err, errDB)
}
