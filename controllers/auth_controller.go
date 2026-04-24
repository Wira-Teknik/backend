package controllers

import (
	"teknik/services"
	"teknik/utils"

	"github.com/gofiber/fiber/v2"
)

// ─────────────────────────────────────────────
// Request DTOs (exported for Swagger)
// ─────────────────────────────────────────────

// RegisterRequest adalah body untuk endpoint register.
type RegisterRequest struct {
	Name     string `json:"name"     example:"Budi Santoso"`
	Email    string `json:"email"    example:"budi@example.com"`
	Password string `json:"password" example:"Password123"`
	Role     string `json:"role"     example:"admin" enums:"admin,owner"`
}

// LoginRequest adalah body untuk endpoint login.
type LoginRequest struct {
	Email    string `json:"email"    example:"budi@example.com"`
	Password string `json:"password" example:"Password123"`
}

// ForgotStep1Request adalah body untuk request OTP.
type ForgotStep1Request struct {
	Email string `json:"email" example:"budi@example.com"`
}

// ForgotStep2Request adalah body untuk verifikasi OTP.
type ForgotStep2Request struct {
	Email string `json:"email" example:"budi@example.com"`
	OTP   string `json:"otp"   example:"482931"`
}

// ForgotStep3Request adalah body untuk reset password.
type ForgotStep3Request struct {
	Token           string `json:"token"            example:"uuid-verified-token"`
	NewPassword     string `json:"new_password"     example:"NewPass456"`
	ConfirmPassword string `json:"confirm_password" example:"NewPass456"`
}

// ─────────────────────────────────────────────
// Register
// POST /api/v1/auth/register
// ─────────────────────────────────────────────

// Register godoc
// @Summary      Register akun baru
// @Description  Membuat akun pengguna baru dengan nama, email, password, dan role
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        body  body      RegisterRequest                          true  "Data registrasi"
// @Success      201   {object}  utils.Response{data=services.UserDTO}   "Akun berhasil dibuat"
// @Failure      400   {object}  utils.Response                          "Validasi gagal atau email sudah terdaftar"
// @Failure      409   {object}  utils.Response                          "Email sudah terdaftar"
// @Router       /auth/register [post]
func Register(c *fiber.Ctx) error {
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "Format request tidak valid")
	}

	result, err := services.RegisterUser(services.RegisterInput{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
		Role:     req.Role,
	})
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}

	return utils.JSONCreated(c, "Akun berhasil dibuat", result)
}

// ─────────────────────────────────────────────
// Login
// POST /api/v1/auth/login
// ─────────────────────────────────────────────

// Login godoc
// @Summary      Login
// @Description  Login menggunakan email dan password. Mengembalikan **JWT** yang dapat digunakan sebagai Bearer token. Cookie HttpOnly juga di-set secara otomatis.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        body  body      LoginRequest                                        true  "Kredensial login"
// @Success      200   {object}  utils.Response{data=controllers.LoginResponseData}  "Login berhasil"
// @Failure      400   {object}  utils.Response                                      "Format request tidak valid"
// @Failure      401   {object}  utils.Response                                      "Email atau password salah"
// @Router       /auth/login [post]
func Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "Format request tidak valid")
	}

	result, err := services.LoginUser(services.LoginInput{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		return utils.JSONError(c, fiber.StatusUnauthorized, err.Error())
	}

	// Set HttpOnly session cookie
	c.Cookie(&fiber.Cookie{
		Name:     "session_token",
		Value:    result.Token,
		MaxAge:   86400, // 24 jam
		HTTPOnly: true,
		SameSite: "Lax",
	})

	return utils.JSONSuccess(c, "Login berhasil", fiber.Map{
		"token": result.Token,
		"user":  result.User,
	})
}

// ─────────────────────────────────────────────
// Swagger-only response model for Login
// ─────────────────────────────────────────────

// LoginResponseData adalah struktur data response login.
// Token adalah JWT yang harus dikirim sebagai: Authorization: Bearer <token>
type LoginResponseData struct {
	Token string           `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	User  services.UserDTO `json:"user"`
}

// ─────────────────────────────────────────────
// Forgot Password – Step 1: Request OTP
// POST /api/v1/auth/forgot-password/request
// ─────────────────────────────────────────────

// ForgotPasswordRequest godoc
// @Summary      Langkah 1 – Request kode OTP
// @Description  Mengirim kode OTP 6 digit ke email pengguna. OTP berlaku 15 menit. Response selalu sukses meski email tidak ditemukan (mencegah user enumeration).
// @Tags         Auth - Forgot Password
// @Accept       json
// @Produce      json
// @Param        body  body      ForgotStep1Request  true  "Email terdaftar"
// @Success      200   {object}  utils.Response      "OTP dikirim (atau email tidak ditemukan, response tetap sama)"
// @Failure      400   {object}  utils.Response      "Format email tidak valid"
// @Router       /auth/forgot-password/request [post]
func ForgotPasswordRequest(c *fiber.Ctx) error {
	var req ForgotStep1Request
	if err := c.BodyParser(&req); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "Format request tidak valid")
	}

	if err := services.ForgotPasswordRequestOTP(req.Email); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}

	return utils.JSONSuccess(c, "Jika email terdaftar, kode verifikasi akan dikirim", nil)
}

// ─────────────────────────────────────────────
// Forgot Password – Step 2: Verify OTP
// POST /api/v1/auth/forgot-password/verify
// ─────────────────────────────────────────────

// ForgotPasswordVerify godoc
// @Summary      Langkah 2 – Verifikasi kode OTP
// @Description  Memvalidasi kode OTP yang diterima via email. Jika valid, mengembalikan token sementara (berlaku 10 menit) untuk mereset password. OTP langsung dihapus setelah berhasil (single-use).
// @Tags         Auth - Forgot Password
// @Accept       json
// @Produce      json
// @Param        body  body      ForgotStep2Request                              true  "Email dan kode OTP"
// @Success      200   {object}  utils.Response{data=controllers.VerifyTokenData} "OTP valid, token verifikasi dikembalikan"
// @Failure      400   {object}  utils.Response                                   "OTP salah atau kadaluarsa"
// @Router       /auth/forgot-password/verify [post]
func ForgotPasswordVerify(c *fiber.Ctx) error {
	var req ForgotStep2Request
	if err := c.BodyParser(&req); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "Format request tidak valid")
	}

	token, err := services.ForgotPasswordVerifyOTP(req.Email, req.OTP)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}

	return utils.JSONSuccess(c, "Kode verifikasi valid. Silakan ubah password Anda", fiber.Map{
		"token": token,
	})
}

// VerifyTokenData adalah struktur data response verifikasi OTP.
type VerifyTokenData struct {
	Token string `json:"token" example:"uuid-verified-token"`
}

// ─────────────────────────────────────────────
// Forgot Password – Step 3: Reset Password
// POST /api/v1/auth/forgot-password/reset
// ─────────────────────────────────────────────

// ForgotPasswordReset godoc
// @Summary      Langkah 3 – Reset password
// @Description  Mereset password menggunakan token yang didapat dari langkah 2. Password baru minimal 8 karakter dan harus mengandung huruf serta angka. Token dihapus setelah digunakan (single-use).
// @Tags         Auth - Forgot Password
// @Accept       json
// @Produce      json
// @Param        body  body      ForgotStep3Request  true  "Token verifikasi dan password baru"
// @Success      200   {object}  utils.Response      "Password berhasil diubah"
// @Failure      400   {object}  utils.Response      "Validasi gagal atau token tidak valid/kadaluarsa"
// @Router       /auth/forgot-password/reset [post]
func ForgotPasswordReset(c *fiber.Ctx) error {
	var req ForgotStep3Request
	if err := c.BodyParser(&req); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "Format request tidak valid")
	}

	if err := services.ForgotPasswordReset(req.Token, req.NewPassword, req.ConfirmPassword); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}

	return utils.JSONSuccess(c, "Kata sandi berhasil diubah! Silakan login dengan password baru Anda.", nil)
}
