package controllers

import (
	"errors"
	"strconv"

	"teknik/services"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// PaginationMeta adalah metadata pagination untuk API response.
type PaginationMeta struct {
	Page       int   `json:"page"       example:"1"`
	Limit      int   `json:"limit"      example:"20"`
	TotalRows  int64 `json:"total_rows" example:"100"`
	TotalPages int   `json:"total_pages" example:"5"`
}

// parsePaginationParams mengambil query param page & limit dengan nilai default.
func parsePaginationParams(c *fiber.Ctx) (page, limit int) {
	page = 1
	if p := c.Query("page"); p != "" {
		if val, err := strconv.Atoi(p); err == nil && val > 0 {
			page = val
		}
	}
	limit = 20
	if l := c.Query("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			limit = val
		}
	}
	return
}

// getAuthorizedUserID mengekstrak dan mem-parse User ID dari Locals secara aman.
func getAuthorizedUserID(c *fiber.Ctx) (uuid.UUID, error) {
	userIDVal := c.Locals("userID")
	if userIDVal == nil {
		return uuid.Nil, errors.New("userID tidak ditemukan dalam request")
	}
	userIDStr, ok := userIDVal.(string)
	if !ok || userIDStr == "" {
		return uuid.Nil, errors.New("userID request tidak valid")
	}
	return services.ParseUserID(userIDStr)
}
