// @title           Wira Teknik API
// @version         1.0
// @description     REST API untuk sistem manajemen Wira Teknik. Mendukung autentikasi, manajemen pengguna, dan fitur bisnis lainnya.
// @termsOfService  http://swagger.io/terms/

// @contact.name   Tim Wira Teknik
// @contact.email  admin@wira-teknik.com

// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT

// @BasePath  /api/v1

// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
// @description                 Masukkan token dengan format: **Bearer &lt;token&gt;**

package main

import (
	"log"
	"os"
	"strings"

	_ "teknik/docs"

	"teknik/config"
	"teknik/models"
	"teknik/routes"

	swagger "github.com/gofiber/swagger"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	fiberlogger "github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using system environment variables")
	}

	// Connect to database (auto-creates DB if missing)
	config.ConnectDatabase()

	// Run AutoMigrate
	if err := config.DB.AutoMigrate(&models.User{}); err != nil {
		log.Fatalf("AutoMigrate failed: %v", err)
	}
	log.Println("Database migration completed.")

	// Connect to Redis
	config.ConnectRedis()

	// Initialize Fiber
	app := fiber.New(fiber.Config{
		AppName:      os.Getenv("APP_NAME"),
		ErrorHandler: customErrorHandler,
	})

	// Middleware: Logger
	app.Use(fiberlogger.New(fiberlogger.Config{
		Format: "[${time}] ${status} - ${latency} ${method} ${path}\n",
	}))

	// Middleware: Recover from panics
	app.Use(recover.New())

	// Middleware: CORS
	corsOrigins := os.Getenv("CORS_ORIGIN")
	allowOrigins := "*"
	if corsOrigins != "" {
		origins := strings.Split(corsOrigins, ",")
		for i := range origins {
			origins[i] = strings.TrimSpace(origins[i])
		}
		allowOrigins = strings.Join(origins, ", ")
	}
	app.Use(cors.New(cors.Config{
		AllowOrigins:     allowOrigins,
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowMethods:     "GET, POST, PUT, PATCH, DELETE, OPTIONS",
		AllowCredentials: true,
	}))

	// Swagger UI route
	app.Get("/swagger/*", swagger.HandlerDefault)

	// Register API routes
	routes.SetupRoutes(app)

	// 404 handler
	app.Use(func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Endpoint tidak ditemukan",
		})
	})

	// Start server
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "7001"
	}

	log.Printf("Server %s berjalan di port %s", os.Getenv("APP_NAME"), port)
	log.Printf("Swagger UI: http://localhost:%s/swagger/index.html", port)

	if err := app.Listen(":" + port); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// customErrorHandler handles unhandled errors with a standard JSON response.
func customErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	message := "Terjadi kesalahan internal"

	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
		message = e.Message
	}

	return c.Status(code).JSON(fiber.Map{
		"success": false,
		"message": message,
	})
}
