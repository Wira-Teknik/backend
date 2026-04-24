package middlewares

import (
	"strings"

	"teknik/utils"

	"github.com/gofiber/fiber/v2"
)

// RequireAuth memvalidasi JWT dari Authorization header atau cookie session_token.
// Claims (UserID, Email, Role) disimpan di Fiber locals untuk handler downstream.
//
// @Security BearerAuth
func RequireAuth(c *fiber.Ctx) error {
	tokenStr := extractToken(c)
	if tokenStr == "" {
		return utils.JSONError(c, fiber.StatusUnauthorized, "Token autentikasi diperlukan")
	}

	claims, err := utils.ValidateJWT(tokenStr)
	if err != nil {
		return utils.JSONError(c, fiber.StatusUnauthorized, "Token tidak valid atau sudah kadaluarsa. Silakan login kembali")
	}

	// Simpan claims ke Fiber locals agar bisa diakses di controller/service
	c.Locals("userID", claims.UserID)
	c.Locals("userEmail", claims.Email)
	c.Locals("userRole", claims.Role)

	return c.Next()
}

// RequireRole membatasi akses hanya untuk role tertentu.
// Harus dipasang setelah RequireAuth.
func RequireRole(roles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userRole, ok := c.Locals("userRole").(string)
		if !ok || userRole == "" {
			return utils.JSONError(c, fiber.StatusForbidden, "Akses ditolak")
		}

		for _, role := range roles {
			if userRole == role {
				return c.Next()
			}
		}

		return utils.JSONError(c, fiber.StatusForbidden, "Anda tidak memiliki izin untuk mengakses resource ini")
	}
}

// extractToken membaca Bearer token dari Authorization header,
// atau fallback ke cookie session_token.
func extractToken(c *fiber.Ctx) string {
	authHeader := c.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	return c.Cookies("session_token")
}
