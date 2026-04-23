package logger

import "strings"

// MaskPlate masks a vehicle plate for logs (PII / LGPD).
// Examples: "ABC1234" -> "ABC-****", "ABC1D23" -> "ABC-****".
func MaskPlate(plate string) string {
	p := strings.TrimSpace(strings.ToUpper(plate))
	if p == "" {
		return ""
	}
	if len(p) <= 3 {
		return "****"
	}
	return p[:3] + "-****"
}
