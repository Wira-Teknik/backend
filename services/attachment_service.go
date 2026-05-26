package services

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"teknik/config"
	"teknik/models"

	"github.com/google/uuid"
)

// Sentinel errors for Attachment service
var (
	ErrAttachmentInvalidRelatedID = errors.New("ID resource terkait tidak valid")
	ErrAttachmentInvalidCategory  = errors.New("kategori lampiran tidak valid")
)

// ─────────────────────────────────────────────
// Constants
// ─────────────────────────────────────────────

const uploadDir = "./uploads"

var allowedCategories = map[string]bool{
	"shipment_delivery": true,
	"shipment_received": true,
	"invoice":           true,
	"payment_proof":     true,
	"bon":               true,
	"surat_jalan":       true,
}

var allowedFileTypes = map[string]bool{
	"image": true,
	"video": true,
	"pdf":   true,
}

// extToFileType memetakan ekstensi file ke tipe file.
var extToFileType = map[string]models.FileType{
	".jpg":  models.FileTypeImage,
	".jpeg": models.FileTypeImage,
	".png":  models.FileTypeImage,
	".gif":  models.FileTypeImage,
	".webp": models.FileTypeImage,
	".mp4":  models.FileTypeVideo,
	".mov":  models.FileTypeVideo,
	".avi":  models.FileTypeVideo,
	".webm": models.FileTypeVideo,
	".pdf":  models.FileTypePdf,
}

// ─────────────────────────────────────────────
// Get Attachments by Related ID
// ─────────────────────────────────────────────

func GetAttachmentsByRelatedID(relatedID string) ([]models.Attachment, error) {
	if _, err := uuid.Parse(relatedID); err != nil {
		return nil, ErrAttachmentInvalidRelatedID
	}

	var attachments []models.Attachment
	err := config.DB.Where("related_id = ?", relatedID).
		Order("created_at DESC").
		Find(&attachments).Error
	if attachments == nil {
		attachments = []models.Attachment{}
	}
	return attachments, err
}

// ─────────────────────────────────────────────
// Upload Attachment
// ─────────────────────────────────────────────

func UploadAttachment(file *multipart.FileHeader, relatedID, category string, userID uuid.UUID) (models.Attachment, error) {
	// Validasi related_id
	relID, err := uuid.Parse(relatedID)
	if err != nil {
		return models.Attachment{}, ErrAttachmentInvalidRelatedID
	}

	// Validasi category
	if !allowedCategories[category] {
		return models.Attachment{}, ErrAttachmentInvalidCategory
	}

	// Deteksi file type dari ekstensi
	ext := strings.ToLower(filepath.Ext(file.Filename))
	fileType, ok := extToFileType[ext]
	if !ok {
		return models.Attachment{}, fmt.Errorf("tipe file tidak didukung (gunakan: jpg, png, gif, webp, mp4, mov, avi, webm, pdf)")
	}

	// Buat direktori upload jika belum ada
	categoryDir := filepath.Join(uploadDir, category)
	if err := os.MkdirAll(categoryDir, 0755); err != nil {
		return models.Attachment{}, fmt.Errorf("gagal membuat direktori upload")
	}

	// Generate nama file unik
	uniqueName := fmt.Sprintf("%s_%d%s", uuid.New().String(), time.Now().UnixMilli(), ext)
	filePath := filepath.Join(categoryDir, uniqueName)

	// Simpan file
	src, err := file.Open()
	if err != nil {
		return models.Attachment{}, fmt.Errorf("gagal membaca file")
	}
	defer src.Close()

	dst, err := os.Create(filePath)
	if err != nil {
		return models.Attachment{}, fmt.Errorf("gagal menyimpan file")
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return models.Attachment{}, fmt.Errorf("gagal menyimpan file")
	}

	// Simpan ke database
	attachment := models.Attachment{
		ID:        uuid.New(),
		RelatedID: relID,
		Category:  models.AttachmentCategory(category),
		FileType:  fileType,
		FileURL:   "/" + filepath.ToSlash(filePath),
	}

	if err := config.DB.Create(&attachment).Error; err != nil {
		// Cleanup file jika gagal simpan ke DB
		os.Remove(filePath)
		return models.Attachment{}, fmt.Errorf("gagal menyimpan data attachment")
	}

	CreateAuditLog(userID, attachment.ID, models.AuditActionUploadDoc, "attachments", nil, attachment)

	return attachment, nil
}
