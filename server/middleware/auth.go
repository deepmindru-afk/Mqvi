package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/akinalp/mqvi/handlers"
	"github.com/akinalp/mqvi/pkg"
	"github.com/akinalp/mqvi/repository"
	"github.com/akinalp/mqvi/services"
)

// AuthMiddleware validates JWT tokens on incoming requests.
type AuthMiddleware struct {
	authService services.AuthService
	userRepo    repository.UserRepository
}

func NewAuthMiddleware(authService services.AuthService, userRepo repository.UserRepository) *AuthMiddleware {
	return &AuthMiddleware{
		authService: authService,
		userRepo:    userRepo,
	}
}

// Require enforces a valid JWT token. Returns 401 if missing or invalid.
func (m *AuthMiddleware) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			pkg.ErrorWithMessage(w, http.StatusUnauthorized, "authorization header required")
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			pkg.ErrorWithMessage(w, http.StatusUnauthorized, "invalid authorization format, use: Bearer <token>")
			return
		}
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		claims, err := m.authService.ValidateAccessToken(tokenString)
		if err != nil {
			pkg.Error(w, err)
			return
		}

		// GetActiveByID rejects soft-deleted/tombstone users so existing access
		// tokens cannot be used after admin or self soft-delete (within token TTL).
		user, err := m.userRepo.GetActiveByID(r.Context(), claims.UserID)
		if err != nil {
			pkg.ErrorWithMessage(w, http.StatusUnauthorized, "user not found")
			return
		}

		// Banned users blocked at middleware too (defense in depth — login already rejects).
		if user.IsPlatformBanned {
			pkg.ErrorWithMessage(w, http.StatusForbidden, "account suspended")
			return
		}

		user.PasswordHash = ""

		ctx := context.WithValue(r.Context(), handlers.UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
