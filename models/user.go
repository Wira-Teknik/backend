package models

import (
	"teknik/utils"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UserRole mendefinisikan peran (role) pengguna yang diizinkan.
type UserRole string

const (
	RoleAdmin UserRole = "admin"
	RoleOwner UserRole = "owner"
)

// User mendefinisikan struktur model pengguna dalam database.
type User struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name      string         `gorm:"type:varchar(255);uniqueIndex;not null" json:"name"`
	Email     string         `gorm:"type:varchar(255);uniqueIndex;not null" json:"email"`
	Password  string         `gorm:"type:varchar(255);not null" json:"-"`
	Role      UserRole       `gorm:"type:varchar(10);not null;default:'admin';check:role IN ('admin','owner')" json:"role"`
	CreatedAt utils.JSONDateTime `json:"created_at"`
	UpdatedAt utils.JSONDateTime `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
