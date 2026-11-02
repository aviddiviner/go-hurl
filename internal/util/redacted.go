package util

import "strings"

// Redact replaces each occurrence of every secret in s with "***".
func Redact(s string, secrets []string) string {
	for _, secret := range secrets {
		s = strings.ReplaceAll(s, secret, "***")
	}
	return s
}
