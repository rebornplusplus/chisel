package setup

import (
	"bytes"
	"fmt"
	"io"

	"golang.org/x/crypto/openpgp/armor"
	"golang.org/x/crypto/openpgp/clearsign"
	"golang.org/x/crypto/openpgp/packet"
)

// DecodeKeys decodes public and private key packets from armored data.
func DecodeKeys(armoredKey []byte) (pubKeys []*packet.PublicKey, privKeys []*packet.PrivateKey, err error) {
	block, err := armor.Decode(bytes.NewReader(armoredKey))
	if err != nil {
		return nil, nil, fmt.Errorf("cannot decode armored data")
	}

	reader := packet.NewReader(block.Body)
	for {
		p, err := reader.Next()
		if err != nil {
			break
		}
		if privKey, ok := p.(*packet.PrivateKey); ok {
			privKeys = append(privKeys, privKey)
		}
		if pubKey, ok := p.(*packet.PublicKey); ok {
			pubKeys = append(pubKeys, pubKey)
		}
	}
	return pubKeys, privKeys, nil
}

// DecodeSinglePublicKey decodes a single public key packet from armored data.
// The data should contain exactly one public key and no private keys.
func DecodeSinglePublicKey(armoredKey []byte) (*packet.PublicKey, error) {
	pubKeys, privKeys, err := DecodeKeys(armoredKey)
	if err != nil {
		return nil, err
	}
	if len(pubKeys) == 0 {
		return nil, fmt.Errorf("no public key packet found")
	}
	if len(pubKeys) > 1 {
		return nil, fmt.Errorf("armored data contains more than one public key packet")
	}
	if len(privKeys) > 0 {
		return nil, fmt.Errorf("armored data contains private key packet")
	}
	return pubKeys[0], nil
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
