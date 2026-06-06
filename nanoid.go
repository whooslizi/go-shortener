package main

import (
	"crypto/rand"
	"math/big"
)

const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"

func GenerateNanoid(length int) (string, error) {
	alphabetLen := big.NewInt(int64(len(alphabet)))
	result := make([]byte, length)

	for i := 0; i < length; i++ {
		idx, err := rand.Int(rand.Reader, alphabetLen)
		if err != nil {
			return "", err
		}
		result[i] = alphabet[idx.Int64()]
	}

	return string(result), nil
}
