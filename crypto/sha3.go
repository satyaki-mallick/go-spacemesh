package crypto

import (
	"golang.org/x/crypto/sha3"
	"github.com/spacemeshos/go-spacemesh/common"
)

// Sha256 is a SHA-3-256 (not sha-256) hasher. It returns a 32 bytes (256 bits) hash of data.
// data: arbitrary length bytes slice
func Sha256(data ...[]byte) []byte {
	d := sha3.New256()
	for _, b := range data {
		d.Write(b)
	}
	return d.Sum(nil)
}

// Keccak256Hash calculates and returns the Keccak256 hash of the input data,
// converting it to an internal Hash data structure.
func Keccak256Hash(data ...[]byte) (h common.Hash) {
	d := sha3.New256()
	for _, b := range data {
		d.Write(b)
	}
	d.Sum(h[:0])
	return h
}
