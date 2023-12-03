package setup

import (
	"bytes"
	"fmt"
	"io"

	"golang.org/x/crypto/openpgp/armor"
	"golang.org/x/crypto/openpgp/clearsign"
	"golang.org/x/crypto/openpgp/packet"
)

// DecodePublicKey decodes a single public key from the armored data.
// The data should contain exactly one public key and no private keys.
// It returns a nil error if a public key can be successfully decoded.
func DecodePublicKey(armoredKey []byte) (*packet.PublicKey, error) {
	block, err := armor.Decode(bytes.NewReader(armoredKey))
	if err != nil {
		return nil, fmt.Errorf("cannot decode armor")
	}

	reader := packet.NewReader(block.Body)
	var pubKey *packet.PublicKey
	for {
		p, err := reader.Next()
		if err != nil {
			break
		}
		if pk, ok := p.(*packet.PublicKey); ok {
			if pubKey == nil {
				pubKey = pk
			} else {
				return nil, fmt.Errorf("armored data contains more than one public key")
			}
		}
		if _, ok := p.(*packet.PrivateKey); ok {
			return nil, fmt.Errorf("armored data should not contain any private keys")
		}
	}
	if pubKey == nil {
		return nil, fmt.Errorf("no public key found")
	}
	return pubKey, nil
}

func DecodeSignature(clearData []byte) (sig *packet.Signature, body []byte, plainText []byte, err error) {
	block, _ := clearsign.Decode(clearData)
	if block == nil {
		return nil, nil, nil, fmt.Errorf("invalid clearsign text")
	}
	reader := packet.NewReader(block.ArmoredSignature.Body)
	p, err := reader.Next()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error reading signature: %w", err)
	}
	sig, ok := p.(*packet.Signature)
	if !ok {
		return nil, nil, nil, fmt.Errorf("error parsing signature")
	}
	return sig, block.Bytes, block.Plaintext, nil
}

func VerifySignature(pubKey *packet.PublicKey, sig *packet.Signature, body []byte) error {
	hash := sig.Hash.New()
	io.Copy(hash, bytes.NewBuffer(body))
	return pubKey.VerifySignature(hash, sig)
}
