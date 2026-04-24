package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"teknik/config"
	"teknik/models"
	"teknik/utils"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// ─────────────────────────────────────────────
// DTOs
// ─────────────────────────────────────────────

type RegisterInput struct {
	Name     string
	Email    string
	Password string
	Role     string
}

type LoginInput struct {
	Email    string
	Password string
}

type LoginResult struct {
	Token string
	User  UserDTO
}

// UserDTO adalah data pengguna yang dikembalikan ke client (tanpa password).
type UserDTO struct {
	ID    uuid.UUID       `json:"id"    example:"550e8400-e29b-41d4-a716-446655440000"`
	Name  string          `json:"name"  example:"Budi Santoso"`
	Email string          `json:"email" example:"budi@example.com"`
	Role  models.UserRole `json:"role"  example:"admin"`
}

// ─────────────────────────────────────────────
// Redis key helpers
// ─────────────────────────────────────────────

func redisOTPKey(email string) string {
	return fmt.Sprintf("otp:forgot:%s", email)
}

func redisVerifiedTokenKey(token string) string {
	return fmt.Sprintf("verified:forgot:%s", token)
}



// ─────────────────────────────────────────────
// Register
// ─────────────────────────────────────────────

func RegisterUser(input RegisterInput) (UserDTO, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Email = strings.TrimSpace(strings.ToLower(input.Email))
	input.Role = strings.TrimSpace(input.Role)

	if input.Name == "" {
		return UserDTO{}, fmt.Errorf("nama tidak boleh kosong")
	}
	if !utils.IsValidEmail(input.Email) {
		return UserDTO{}, fmt.Errorf("format email tidak valid")
	}
	if !utils.IsStrongPassword(input.Password) {
		return UserDTO{}, fmt.Errorf("password minimal 8 karakter dan harus mengandung huruf serta angka")
	}
	if input.Role != string(models.RoleAdmin) && input.Role != string(models.RoleOwner) {
		return UserDTO{}, fmt.Errorf("role harus 'admin' atau 'owner'")
	}

	var count int64
	config.DB.Model(&models.User{}).Where("email = ?", input.Email).Count(&count)
	if count > 0 {
		return UserDTO{}, fmt.Errorf("email sudah terdaftar")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return UserDTO{}, fmt.Errorf("gagal memproses password")
	}

	user := models.User{
		ID:       uuid.New(),
		Name:     input.Name,
		Email:    input.Email,
		Password: string(hashed),
		Role:     models.UserRole(input.Role),
	}

	if err := config.DB.Create(&user).Error; err != nil {
		return UserDTO{}, fmt.Errorf("gagal membuat akun")
	}

	return UserDTO{ID: user.ID, Name: user.Name, Email: user.Email, Role: user.Role}, nil
}

// ─────────────────────────────────────────────
// Login
// ─────────────────────────────────────────────

func LoginUser(input LoginInput) (LoginResult, error) {
	input.Email = strings.TrimSpace(strings.ToLower(input.Email))

	if !utils.IsValidEmail(input.Email) {
		return LoginResult{}, fmt.Errorf("format email tidak valid")
	}
	if input.Password == "" {
		return LoginResult{}, fmt.Errorf("password tidak boleh kosong")
	}

	var user models.User
	if err := config.DB.Select("id, name, email, password, role").
		Where("email = ?", input.Email).
		First(&user).Error; err != nil {
		return LoginResult{}, fmt.Errorf("email atau password salah")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
		return LoginResult{}, fmt.Errorf("email atau password salah")
	}

	// Generate stateless JWT — tidak perlu simpan di Redis
	jwtToken, err := utils.GenerateJWT(user.ID.String(), user.Email, string(user.Role))
	if err != nil {
		return LoginResult{}, fmt.Errorf("gagal membuat token: %w", err)
	}

	return LoginResult{
		Token: jwtToken,
		User:  UserDTO{ID: user.ID, Name: user.Name, Email: user.Email, Role: user.Role},
	}, nil
}

// ─────────────────────────────────────────────
// Forgot Password – Step 1: Request OTP
// ─────────────────────────────────────────────

// ForgotPasswordRequestOTP checks the email, generates an OTP, stores it in Redis
// and sends it via email. Returns a generic error to prevent user enumeration.
func ForgotPasswordRequestOTP(email string) error {
	email = strings.TrimSpace(strings.ToLower(email))

	if !utils.IsValidEmail(email) {
		return fmt.Errorf("format email tidak valid")
	}

	var user models.User
	if err := config.DB.Select("id, email").Where("email = ?", email).First(&user).Error; err != nil {
		// Return nil (generic success) to prevent user enumeration
		return nil
	}

	otp, err := utils.GenerateOTP()
	if err != nil {
		return fmt.Errorf("gagal membuat kode verifikasi")
	}

	ctx := context.Background()
	if err := config.Redis.Set(ctx, redisOTPKey(email), otp, 15*time.Minute).Err(); err != nil {
		return fmt.Errorf("gagal menyimpan kode verifikasi")
	}

	if sendErr := utils.SendOTPEmail(email, otp); sendErr != nil {
		fmt.Printf("[mailer] error sending OTP to %s: %v\n", email, sendErr)
		return fmt.Errorf("gagal mengirim email. coba lagi nanti")
	}

	return nil
}

// ─────────────────────────────────────────────
// Forgot Password – Step 2: Verify OTP
// ─────────────────────────────────────────────

// ForgotPasswordVerifyOTP validates the OTP and returns a short-lived verified token.
func ForgotPasswordVerifyOTP(email, otp string) (string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	otp = strings.TrimSpace(otp)

	if !utils.IsValidEmail(email) || otp == "" {
		return "", fmt.Errorf("email dan kode verifikasi diperlukan")
	}

	ctx := context.Background()
	storedOTP, err := config.Redis.Get(ctx, redisOTPKey(email)).Result()
	if err != nil {
		return "", fmt.Errorf("kode verifikasi salah atau sudah kadaluarsa")
	}

	if storedOTP != otp {
		return "", fmt.Errorf("kode verifikasi salah atau sudah kadaluarsa")
	}

	// Delete OTP immediately — single use
	config.Redis.Del(ctx, redisOTPKey(email))

	verifiedToken := uuid.New().String()
	if err := config.Redis.Set(ctx, redisVerifiedTokenKey(verifiedToken), email, 10*time.Minute).Err(); err != nil {
		return "", fmt.Errorf("gagal membuat token verifikasi")
	}

	return verifiedToken, nil
}

// ─────────────────────────────────────────────
// Forgot Password – Step 3: Reset Password
// ─────────────────────────────────────────────

// ForgotPasswordReset validates the verified token and updates the user's password.
func ForgotPasswordReset(token, newPassword, confirmPassword string) error {
	token = strings.TrimSpace(token)

	if token == "" {
		return fmt.Errorf("token verifikasi diperlukan")
	}
	if newPassword != confirmPassword {
		return fmt.Errorf("kata sandi tidak cocok")
	}
	if !utils.IsStrongPassword(newPassword) {
		return fmt.Errorf("password minimal 8 karakter dan harus mengandung huruf serta angka")
	}

	ctx := context.Background()
	email, err := config.Redis.Get(ctx, redisVerifiedTokenKey(token)).Result()
	if err != nil {
		return fmt.Errorf("sesi verifikasi tidak valid atau sudah kadaluarsa")
	}

	var user models.User
	if err := config.DB.Select("id, email").Where("email = ?", email).First(&user).Error; err != nil {
		return fmt.Errorf("akun tidak ditemukan")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("gagal memproses password")
	}

	if err := config.DB.Model(&user).Update("password", string(hashed)).Error; err != nil {
		return fmt.Errorf("gagal memperbarui password")
	}

	// Delete verified token — single use
	config.Redis.Del(ctx, redisVerifiedTokenKey(token))

	return nil
}
