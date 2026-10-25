package services_test

import (
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/services"
)

func TestHouseholdService_ExpiredInvite_Gone(t *testing.T) {
	expiredToken := &domain.UserToken{
		Token:   "EXPIRED1",
		Type:    domain.TokenTypeHouseholdInvite,
		Expires: time.Now().Add(-time.Hour),
		User:    &domain.User{ID: uuid.New(), HouseholdID: uuid.New()},
	}
	userRepo := &stubUserRepo{
		findTokenFn: func(_, _ string) (*domain.UserToken, error) { return expiredToken, nil },
	}
	svc := services.NewHouseholdService(&stubHouseholdRepo{}, userRepo, nil)

	// Join is a protected route: a 401 here would make clients drop a valid session.
	_, joinErr := svc.JoinByInvite(uuid.New(), "EXPIRED1")
	_, infoErr := svc.InviteInfo("EXPIRED1")

	for _, err := range []error{joinErr, infoErr} {
		var se *sentinels.Error
		require.ErrorAs(t, err, &se)
		assert.Equal(t, fiber.StatusGone, se.Status)
	}
}
