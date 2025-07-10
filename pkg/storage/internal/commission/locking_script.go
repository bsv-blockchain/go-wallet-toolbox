package commission

import (
	"fmt"

	primitives "github.com/bsv-blockchain/go-sdk/primitives/ec"
	crypto "github.com/bsv-blockchain/go-sdk/primitives/hash"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction/template/p2pkh"
)

// ScriptGenerator is a tool to generate a locking script with key offset.
type ScriptGenerator struct {
	offsetPrivGenerator func() (*primitives.PrivateKey, error)
	pubKey              string
}

// NewScriptGenerator creates a new instance of ScriptGenerator.
func NewScriptGenerator(pubKey string) *ScriptGenerator {
	return &ScriptGenerator{
		offsetPrivGenerator: randomPrivateKey,
		pubKey:              pubKey,
	}
}

// SetOffsetGenerator sets the generator function for the offset private key.
// Default is a random private key generator.
func (l *ScriptGenerator) SetOffsetGenerator(generator func() (*primitives.PrivateKey, error)) {
	l.offsetPrivGenerator = generator
}

// Generate creates a locking script and randomizes a key offset (WIF formatted private key) from the given public key.
// NOTE: It is used to add Service Charge output to the transaction.
func (l *ScriptGenerator) Generate() (lockingScript *script.Script, keyOffset string, err error) {
	offsetPub, keyOffset, err := l.offsetPubKey()
	if err != nil {
		return nil, "", err
	}

	address, err := script.NewAddressFromPublicKey(offsetPub, true)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create address from public key: %w", err)
	}

	lockingScriptObj, err := p2pkh.Lock(address)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create locking script: %w", err)
	}

	return lockingScriptObj, keyOffset, nil
}

func (l *ScriptGenerator) offsetPubKey() (offsetPubKey *primitives.PublicKey, keyOffset string, err error) {
	pub, err := primitives.PublicKeyFromString(l.pubKey)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse public key: %w", err)
	}

	hashedSecret, keyOffset, err := l.keyOffsetToHashedSecret(pub)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get hashed secret: %w", err)
	}

	newPointX, newPointY := primitives.S256().ScalarBaseMult(hashedSecret)
	newPubKeyX, newPubKeyY := primitives.S256().Add(newPointX, newPointY, pub.X, pub.Y)
	offsetPubKey = &primitives.PublicKey{
		Curve: primitives.S256(),
		X:     newPubKeyX,
		Y:     newPubKeyY,
	}

	return offsetPubKey, keyOffset, nil
}

func (l *ScriptGenerator) keyOffsetToHashedSecret(pub *primitives.PublicKey) (hashedSecret []byte, keyOffset string, err error) {
	offset, err := l.offsetPrivGenerator()
	if err != nil {
		return nil, "", fmt.Errorf("failed to create new private key for keyOffset: %w", err)
	}

	sharedSecret, err := offset.DeriveSharedSecret(pub)
	if err != nil {
		return nil, "", fmt.Errorf("failed to derive shared secret: %w", err)
	}

	hashedSecret = crypto.Sha256(sharedSecret.ToDER())

	return hashedSecret, offset.Wif(), nil
}

func randomPrivateKey() (*primitives.PrivateKey, error) {
	privKey, err := primitives.NewPrivateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}
	return privKey, nil
}
