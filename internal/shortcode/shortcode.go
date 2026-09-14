package shortcode

import (
	"crypto/rand"
	"errors"
)

const base62Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// RandomBase62 returns a cryptographically random Base62 code.
func RandomBase62(length int) (string, error) {
	if length <= 0 {
		return "", errors.New("code length must be positive")
	}

	code := make([]byte, length)
	buffer := make([]byte, length)

	for filled := 0; filled < length; {
		if _, err := rand.Read(buffer); err != nil {
			return "", err
		}

		for _, value := range buffer {
			if value >= 248 {
				continue
			}

			code[filled] = base62Alphabet[int(value)%len(base62Alphabet)]
			filled++
			if filled == length {
				break
			}
		}
	}

	return string(code), nil
}
