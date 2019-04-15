package signing

import (
	"errors"
	"github.com/btcsuite/btcutil/base58"
	"github.com/spacemeshos/ed25519"
	"github.com/spacemeshos/go-spacemesh/log"
)

type BlsFetcher interface {
	FetchBlsKey(edKey []byte) []byte
}

type ActivationValidator interface {
	IsActive(blsKey []byte) bool
}

type PublicKey struct {
	pub []byte
}

func NewPublicKey(pub []byte) *PublicKey {
	return &PublicKey{pub}
}

func (p *PublicKey) Bytes() []byte {
	return p.pub
}

func (p *PublicKey) String() string {
	return base58.Encode(p.Bytes())
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

func (es *EdSigner) PublicKey() *PublicKey {
	return NewPublicKey(es.pubKey)
}

func (es *EdSigner) ToBuffer() []byte {
	buff := make([]byte, len(es.privKey))
	copy(buff, es.privKey)

	return buff
}