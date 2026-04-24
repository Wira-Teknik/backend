package routes

import (
	"teknik/controllers"

	"github.com/gofiber/fiber/v2"
)

// SetupRoutes registers all API routes.
func SetupRoutes(app *fiber.App) {
	api := app.Group("/api/v1")

	// Auth routes (public)
	auth := api.Group("/auth")
	auth.Post("/register", controllers.Register)
	auth.Post("/login", controllers.Login)
	auth.Post("/forgot-password/request", controllers.ForgotPasswordRequest)
	auth.Post("/forgot-password/verify", controllers.ForgotPasswordVerify)
	auth.Post("/forgot-password/reset", controllers.ForgotPasswordReset)
}
