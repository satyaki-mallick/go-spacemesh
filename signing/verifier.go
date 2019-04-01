package signing

import (
	"github.com/btcsuite/btcutil/base58"
	"github.com/spacemeshos/ed25519"
)

type Verifier interface {
	Verify(data []byte, sig []byte) (bool, error)
	Bytes() []byte
	String() string
}

type edVerifier struct {
	pubKey []byte
}

func NewVerifier(pub []byte) (*edVerifier, error) {
	return &edVerifier{pubKey: pub}, nil
}

func (ev *edVerifier) Verify(data []byte, sig []byte) (bool, error) {
	return ed25519.Verify2(ev.pubKey, data, sig), nil
}

func (ev *edVerifier) Bytes() []byte {
	return ev.pubKey
}

func (ev *edVerifier) String() string {
	return base58.Encode(ev.Bytes())
}
