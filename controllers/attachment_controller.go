package controllers

import (
	"errors"

	"teknik/services"
	"teknik/utils"

	"github.com/gofiber/fiber/v2"
)

// ─────────────────────────────────────────────
// Get Attachments by Related ID
// ─────────────────────────────────────────────

// GetAttachments menangani permintaan untuk mengambil semua lampiran berdasarkan related ID.
// GetAttachments godoc
// @Summary      Ambil semua lampiran berdasarkan related ID
// @Description  Mengambil daftar lampiran (foto/video/pdf) yang terkait dengan resource tertentu (shipment, invoice, payment, dll)
// @Tags         Attachments
// @Param        relatedId  path      string  true  "Related ID (Shipment ID, Invoice ID, dll)"
// @Produce      json
// @Success      200  {object}  utils.Response{data=[]models.Attachment}
// @Router       /attachments/{relatedId} [get]
// @Security     BearerAuth
func GetAttachments(c *fiber.Ctx) error {
	relatedID := c.Params("relatedId")
	attachments, err := services.GetAttachmentsByRelatedID(relatedID)
	if err != nil {
		if errors.Is(err, services.ErrAttachmentInvalidRelatedID) {
			return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
		}
		return utils.JSONError(c, fiber.StatusInternalServerError, "Gagal mengambil data lampiran")
	}
	return utils.JSONSuccess(c, "Data lampiran berhasil diambil", attachments)
}

// ─────────────────────────────────────────────
// Upload Attachment
// ─────────────────────────────────────────────

// UploadAttachment menangani pengunggahan file lampiran dan menyimpannya ke database.
// UploadAttachment godoc
// @Summary      Upload lampiran
// @Description  Mengunggah file lampiran (image/video/pdf) yang terkait dengan resource tertentu. Gunakan form-data dengan field: file, related_id, category. Kategori yang tersedia: shipment_delivery, shipment_received, invoice, payment_proof, bon, surat_jalan.
// @Tags         Attachments
// @Accept       multipart/form-data
// @Produce      json
// @Param        file        formData  file    true  "File yang akan diunggah"
// @Param        related_id  formData  string  true  "ID resource terkait"
// @Param        category    formData  string  true  "Kategori lampiran" Enums(shipment_delivery, shipment_received, invoice, payment_proof, bon, surat_jalan)
// @Success      201  {object}  utils.Response{data=models.Attachment}
// @Failure      400  {object}  utils.Response
// @Router       /attachments [post]
// @Security     BearerAuth
func UploadAttachment(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "File tidak ditemukan dalam request")
	}

	relatedID := c.FormValue("related_id")
	category := c.FormValue("category")

	userID, err := getAuthorizedUserID(c)
	if err != nil {
		return utils.JSONError(c, fiber.StatusUnauthorized, err.Error())
	}

	attachment, err := services.UploadAttachment(file, relatedID, category, userID)
	if err != nil {
		if errors.Is(err, services.ErrAttachmentInvalidRelatedID) || errors.Is(err, services.ErrAttachmentInvalidCategory) {
			return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
		}
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}

	return utils.JSONCreated(c, "Lampiran berhasil diunggah", attachment)
}
