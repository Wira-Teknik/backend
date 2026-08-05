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
	"flag"
	"log"
	"os"
	"strings"
	"time"

	_ "teknik/docs"

	"teknik/config"
	"teknik/models"
	"teknik/routes"
	"teknik/services"

	swagger "github.com/gofiber/swagger"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	fiberlogger "github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/pprof"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"
)

func init() {
	// Mengatur zona waktu default ke Asia/Jakarta (+7)
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err == nil {
		time.Local = loc
	} else {
		time.Local = time.FixedZone("WIB", 7*3600)
	}
}

func main() {
	// Memuat variabel lingkungan (environment variables)
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using system environment variables")
	}

	// Flag baris perintah (command-line flags)
	dropDbFlag := flag.Bool("drop-db", false, "Drop the target database completely")
	resetDbFlag := flag.Bool("reset-db", false, "Reset database schema and re-run migrations/seeding")
	flag.Parse()

	if *dropDbFlag {
		if err := config.DropDatabase(); err != nil {
			log.Fatalf("Error dropping database: %v", err)
		}
		os.Exit(0)
	}

	if *resetDbFlag {
		config.ConnectDatabase()
		config.ResetDatabase()

		log.Println("Running AutoMigrate after schema reset...")
		runMigration()
		log.Println("Database migration completed after reset.")
		log.Println("Database reset and migrated successfully.")
		os.Exit(0)
	}

	// Menghubungkan ke database (membuat DB otomatis jika belum ada)
	config.ConnectDatabase()

	// Menjalankan AutoMigrate
	runMigration()
	log.Println("Database migration completed.")

	// Menghubungkan ke Redis
	config.ConnectRedis()

	// Menginisialisasi Fiber
	app := fiber.New(fiber.Config{
		AppName:      os.Getenv("APP_NAME"),
		BodyLimit:    100 * 1024 * 1024, // Set limit to 100 MB
		ErrorHandler: customErrorHandler,
	})

	// Middleware: Logger (Pencatat Log)
	app.Use(fiberlogger.New(fiberlogger.Config{
		Format: "[${time}] ${status} - ${latency} ${method} ${path}\n",
	}))

	// Middleware: Memulihkan dari kepanikan (panics) / crash
	app.Use(recover.New())

	// Middleware: Pprof (Pembuatan profil Memori & CPU)
	app.Use(pprof.New())

	// Middleware: CORS (Cross-Origin Resource Sharing)
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

	// Rute Swagger UI untuk dokumentasi API
	app.Get("/api/docs/*", swagger.HandlerDefault)

	// Melayani file statis (seperti gambar unggahan)
	app.Static("/uploads", "./uploads")

	// Mendaftarkan rute-rute API
	routes.SetupRoutes(app)

	// Penanganan rute 404 jika tidak ditemukan
	app.Use(func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Endpoint tidak ditemukan",
		})
	})

	// Memulai server HTTP
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "7001"
	}

	log.Printf("Server %s berjalan di port %s", os.Getenv("APP_NAME"), port)
	log.Printf("Swagger UI: http://localhost:%s/api/docs/index.html", port)

	// Memulai pekerjaan cron latar belakang untuk pengingat pembayaran jatuh tempo 3 bulan
	go func() {
		// Menunggu 10 detik setelah startup agar inisialisasi koneksi database selesai sempurna
		time.Sleep(10 * time.Second)

		// Jalankan scan awal saat server pertama kali dinyalakan
		log.Println("[Cron Worker] Menjalankan pengecekan awal untuk pengingat jatuh tempo 3 bulan...")
		count, err := services.Send3MonthOverduePaymentReminders()
		if err != nil {
			log.Printf("[Cron Worker] Pengecekan awal gagal: %v\n", err)
		} else {
			log.Printf("[Cron Worker] Pengecekan awal selesai, %d email terkirim.\n", count)
		}

		// Jalankan secara periodik setiap 24 jam sekali
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for range ticker.C {
			log.Println("[Cron Worker] Menjalankan pengecekan berkala untuk pengingat jatuh tempo 3 bulan...")
			count, err := services.Send3MonthOverduePaymentReminders()
			if err != nil {
				log.Printf("[Cron Worker] Pengecekan berkala gagal: %v\n", err)
			} else {
				log.Printf("[Cron Worker] Pengecekan berkala selesai, %d email terkirim.\n", count)
			}
		}
	}()

	if err := app.Listen(":" + port); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// runMigration menjalankan proses AutoMigrate GORM untuk semua model.
func runMigration() {
	if err := config.DB.AutoMigrate(
		&models.User{},
		&models.Customer{},
		&models.Order{},
		&models.OrderItem{},
		&models.Payment{},
		&models.PaymentDetail{},
		&models.Shipment{},
		&models.ShipmentItem{},
		&models.Invoice{},
		&models.Attachment{},
		&models.AuditLog{},
	); err != nil {
		log.Fatalf("AutoMigrate failed: %v", err)
	}
}

// customErrorHandler menangani kesalahan yang tidak tertangani dengan memberikan respons JSON standar.
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
