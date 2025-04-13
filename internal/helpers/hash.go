package helpers

import (
	"crypto/sha256"
)

func TinyHash(input string) string {
	hash := sha256.Sum256([]byte(input))

	// Take the first 4 bytes from the hash and convert to an integer
	hashInt := int(hash[0])<<24 | int(hash[1])<<16 | int(hash[2])<<8 | int(hash[3])

	// Encode the integer as base62
	return base62Encode(hashInt)
}

func base62Encode(num int) string {
	const charset = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	var result []byte
	for num > 0 {
		result = append([]byte{charset[num%62]}, result...)
		num /= 62
	}
	return string(result)
}
