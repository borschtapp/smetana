package services_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/services"
)

var errDB = errors.New("db error")

func newTestUserService(userRepo *stubUserRepo, householdRepo *stubHouseholdRepo) domain.UserService {
	return services.NewUserService(userRepo, householdRepo)
}

func TestUserService_Delete_DifferentRequester_Forbidden(t *testing.T) {
	svc := newTestUserService(&stubUserRepo{}, &stubHouseholdRepo{})

	err := svc.Delete(uuid.New(), uuid.New())

	require.ErrorIs(t, err, sentinels.ErrForbidden)
}

func TestUserService_Delete_UserNotFound_ReturnsError(t *testing.T) {
	id := uuid.New()
	userRepo := &stubUserRepo{
		byIDFn: func(_ uuid.UUID) (*domain.User, error) { return nil, sentinels.ErrNotFound },
	}

	svc := newTestUserService(userRepo, &stubHouseholdRepo{})
	err := svc.Delete(id, id)

	require.ErrorIs(t, err, sentinels.ErrNotFound)
}

func TestUserService_Delete_LastMember_DeletesHousehold(t *testing.T) {
	userID := uuid.New()
	householdID := uuid.New()
	user := &domain.User{ID: userID, HouseholdID: householdID}

	var deletedHouseholdID uuid.UUID
	userRepo := &stubUserRepo{
		byIDFn:   func(_ uuid.UUID) (*domain.User, error) { return user, nil },
		deleteFn: func(_ uuid.UUID) error { return nil },
	}
	householdRepo := &stubHouseholdRepo{
		membersFn: func(_ uuid.UUID, _, _ int) ([]domain.User, int64, error) { return nil, 0, nil },
		deleteFn:  func(id uuid.UUID) error { deletedHouseholdID = id; return nil },
	}

	svc := newTestUserService(userRepo, householdRepo)
	err := svc.Delete(userID, userID)

	require.NoError(t, err)
	assert.Equal(t, householdID, deletedHouseholdID, "orphaned household must be deleted")
}

func TestUserService_Delete_OtherMembersExist_KeepsHousehold(t *testing.T) {
	userID := uuid.New()
	householdID := uuid.New()
	user := &domain.User{ID: userID, HouseholdID: householdID}

	householdDeleted := false
	userRepo := &stubUserRepo{
		byIDFn:   func(_ uuid.UUID) (*domain.User, error) { return user, nil },
		deleteFn: func(_ uuid.UUID) error { return nil },
	}
	householdRepo := &stubHouseholdRepo{
		membersFn: func(_ uuid.UUID, _, _ int) ([]domain.User, int64, error) {
			return []domain.User{{ID: uuid.New()}}, 1, nil
		},
		deleteFn: func(_ uuid.UUID) error { householdDeleted = true; return nil },
	}

	svc := newTestUserService(userRepo, householdRepo)
	err := svc.Delete(userID, userID)

	require.NoError(t, err)
	assert.False(t, householdDeleted, "household with remaining members must not be deleted")
}

func TestUserService_Delete_UserDeleteFails_ReturnsError(t *testing.T) {
	userID := uuid.New()
	user := &domain.User{ID: userID, HouseholdID: uuid.New()}

	userRepo := &stubUserRepo{
		byIDFn:   func(_ uuid.UUID) (*domain.User, error) { return user, nil },
		deleteFn: func(_ uuid.UUID) error { return errDB },
	}

	svc := newTestUserService(userRepo, &stubHouseholdRepo{})
	err := svc.Delete(userID, userID)

	require.ErrorIs(t, err, errDB)
}

func TestUserService_Delete_MembersCheckFails_ReturnsError(t *testing.T) {
	userID := uuid.New()
	user := &domain.User{ID: userID, HouseholdID: uuid.New()}

	userRepo := &stubUserRepo{
		byIDFn:   func(_ uuid.UUID) (*domain.User, error) { return user, nil },
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

func TestUserService_Delete_MembersEmptyButHouseholdDeleteFails_ReturnsError(t *testing.T) {
	userID := uuid.New()
	user := &domain.User{ID: userID, HouseholdID: uuid.New()}

	userRepo := &stubUserRepo{
		byIDFn:   func(_ uuid.UUID) (*domain.User, error) { return user, nil },
		deleteFn: func(_ uuid.UUID) error { return nil },
	}
	householdRepo := &stubHouseholdRepo{
		membersFn: func(_ uuid.UUID, _, _ int) ([]domain.User, int64, error) { return nil, 0, nil },
		deleteFn:  func(_ uuid.UUID) error { return errDB },
	}

	svc := newTestUserService(userRepo, householdRepo)
	err := svc.Delete(userID, userID)

	require.ErrorIs(t, err, errDB)
}
