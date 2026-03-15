package utils

import (
	"crypto/rand"
	"encoding/base64"
	"math/big"
)

func GenerateRandomString(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return base64.URLEncoding.EncodeToString(b)[:length]
}

func GenerateInviteCode() string {
	const alphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"
	const length = 8
	result := make([]byte, length)
	for i := range result {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return ""
		}
		result[i] = alphabet[n.Int64()]
	}
	return string(result)
}
