package service

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
)

// ValidationError memetakan nama field ke pesan kesalahannya.
type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string {
	keys := make([]string, 0, len(e.Fields))
	for k := range e.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%s: %s", k, e.Fields[k])
	}
	return "validasi gagal: " + strings.Join(parts, "; ")
}

type validator struct{ fields map[string]string }

func (v *validator) add(field, msg string) {
	if v.fields == nil {
		v.fields = map[string]string{}
	}
	if _, exists := v.fields[field]; !exists {
		v.fields[field] = msg
	}
}

func (v *validator) err() error {
	if len(v.fields) == 0 {
		return nil
	}
	return &ValidationError{Fields: v.fields}
}

// ConflictError membawa id resource yang sudah ada, misal lowongan dengan source_url sama.
type ConflictError struct {
	Message    string
	ExistingID string
}

func (e *ConflictError) Error() string { return e.Message }
func (e *ConflictError) Unwrap() error { return ErrConflict }
