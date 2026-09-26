package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ahmadhasan/ai-career-compass/backend/internal/httpx"
	"github.com/ahmadhasan/ai-career-compass/backend/internal/service"
)

// decodeJSON membaca tepat satu objek JSON dan menolak field yang tidak dikenal.
// Mengembalikan false jika respons error sudah ditulis.
func decodeJSON(c *gin.Context, dst any) bool {
	dec := json.NewDecoder(c.Request.Body)
	dec.DisallowUnknownFields()
	err := dec.Decode(dst)
	if err == nil && dec.More() {
		err = errors.New("body berisi lebih dari satu objek JSON")
	}
	if err == nil {
		return true
	}

	var (
		maxErr    *http.MaxBytesError
		typeErr   *json.UnmarshalTypeError
		syntaxErr *json.SyntaxError
	)
	switch {
	case errors.As(err, &maxErr):
		httpx.AbortError(c, http.StatusRequestEntityTooLarge, "body_too_large",
			fmt.Sprintf("body melebihi %d byte", maxErr.Limit))
	case errors.As(err, &typeErr):
		httpx.AbortValidation(c, http.StatusBadRequest, "tipe data tidak sesuai",
			map[string]string{typeErr.Field: "harus bertipe " + typeErr.Type.String()})
	case errors.As(err, &syntaxErr), errors.Is(err, io.ErrUnexpectedEOF):
		httpx.AbortError(c, http.StatusBadRequest, "invalid_json", "JSON tidak valid")
	case errors.Is(err, io.EOF):
		httpx.AbortError(c, http.StatusBadRequest, "invalid_json", "body JSON wajib diisi")
	default:
		// termasuk "json: unknown field ..."
		httpx.AbortError(c, http.StatusBadRequest, "invalid_json", err.Error())
	}
	return false
}

// respondError memetakan error service ke status HTTP.
func respondError(c *gin.Context, err error) {
	var (
		valErr      *service.ValidationError
		conflictErr *service.ConflictError
	)
	switch {
	case errors.As(err, &valErr):
		httpx.AbortValidation(c, http.StatusUnprocessableEntity, "input tidak valid", valErr.Fields)
	case errors.As(err, &conflictErr):
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{
			"error":       httpx.ErrorBody{Code: "conflict", Message: conflictErr.Message},
			"existing_id": conflictErr.ExistingID,
		})
	case errors.Is(err, service.ErrNotFound):
		httpx.AbortError(c, http.StatusNotFound, "not_found", "data tidak ditemukan")
	default:
		_ = c.Error(err)
		httpx.AbortError(c, http.StatusInternalServerError, "internal_error", "terjadi kesalahan pada server")
	}
}
