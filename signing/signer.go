package signing

import (
	"errors"
	"github.com/spacemeshos/ed25519"
	"github.com/spacemeshos/go-spacemesh/log"
)

type BlsFetcher interface {
	FetchBlsKey(edKey []byte) []byte
}

type ActivationValidator interface {
	IsActive(blsKey []byte) bool
}

type Signer interface {
	Sign(m []byte) []byte
	Verifier() Verifier
}

type EdSigner struct {
	privKey ed25519.PrivateKey // the pub & private key
	pubKey  ed25519.PublicKey  // only the pub key part
}

func NewEdSignerFromBuffer(buff []byte) (*EdSigner, error) {
	if len(buff) < 32 {
		return nil, errors.New("buffer too small")
	}
	return &EdSigner{privKey: buff, pubKey: buff[:32]}, nil
}

func NewEdSigner() *EdSigner {
	pub, priv, err := ed25519.GenerateKey(nil)

	if err != nil {
		log.Panic("Could not generate key pair err=%v", err)
	}

	return &EdSigner{privKey: priv, pubKey: pub}
}

func (es *EdSigner) Sign(m []byte) []byte {
	return ed25519.Sign2(es.privKey, m)
}

func (es *EdSigner) Verifier() Verifier {
	v, _ := NewVerifier([]byte(es.pubKey))
	return v
}

func (es *EdSigner) ToBuffer() []byte {
	buff := make([]byte, len(es.privKey))
	copy(buff, es.privKey)

	return buff
}