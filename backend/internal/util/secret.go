package util

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

func HashCardCode(code string) string {
	sum := sha256.Sum256([]byte(normalizeCardCode(code)))
	return hex.EncodeToString(sum[:])
}

func MaskCardCode(code string) string {
	value := strings.TrimSpace(code)
	if len(value) <= 8 {
		return value
	}
	return value[:4] + "****" + value[len(value)-4:]
}

func GenerateCardCode() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	raw := strings.ToUpper(hex.EncodeToString(b))
	return fmt.Sprintf("SMS-%s-%s-%s-%s", raw[:6], raw[6:12], raw[12:18], raw[18:24]), nil
}

func GenerateOrderNo() string {
	return "ORD" + time.Now().Format("20060102150405") + strings.ToUpper(strings.ReplaceAll(uuid.NewString()[:8], "-", ""))
}

func normalizeCardCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}
