package service

import (
	"slices"
	"strings"
	"unicode/utf8"
)

// cleanOptional memangkas spasi; string kosong menjadi nil.
func cleanOptional(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}

func tooLong(s *string, max int) bool {
	return s != nil && utf8.RuneCountInString(*s) > max
}

func oneOf(s *string, allowed []string) bool {
	return s == nil || slices.Contains(allowed, *s)
}
