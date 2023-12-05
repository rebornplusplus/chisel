package testutil

import (
	"log"

	"golang.org/x/crypto/openpgp/packet"

	"github.com/canonical/chisel/internal/setup"
)

type Key struct {
	ID                   string
	ArmoredPublicKey     string
	ArmoredPrivateKey    string
	PrivateKeyPassphrase string
	PublicKey            *packet.PublicKey
	PrivateKey           *packet.PrivateKey
}

var gpgKeys = map[string]*Key{
	"ubuntu-archive-key": {
		ID:               "871920D1991BC93C",
		ArmoredPublicKey: ubuntuArchiveSignKey2018,
	},
	"test-key": {
		ID:                "854BAF1AA9D76600",
		ArmoredPublicKey:  testPublicKeyData,
		ArmoredPrivateKey: testPrivateKeyData,
	},
}

func init() {
	for name, key := range gpgKeys {
		if key.ArmoredPublicKey != "" {
			pubKeys, privKeys, err := setup.DecodeKeys([]byte(key.ArmoredPublicKey))
			if err != nil || len(privKeys) > 0 || len(pubKeys) != 1 || pubKeys[0].KeyIdString() != key.ID {
				log.Panicf("invalid public key armored data: %s", name)
			}
			key.PublicKey = pubKeys[0]
		}
		if key.ArmoredPrivateKey != "" {
			pubKeys, privKeys, err := setup.DecodeKeys([]byte(key.ArmoredPrivateKey))
			if err != nil || len(pubKeys) > 0 || len(privKeys) != 1 || privKeys[0].KeyIdString() != key.ID {
				log.Println(len(pubKeys), len(privKeys), err)
				log.Panicf("invalid private key armored data: %s", name)
			}
			key.PrivateKey = privKeys[0]
			if key.PrivateKeyPassphrase != "" {
				err = key.PrivateKey.Decrypt([]byte(key.PrivateKeyPassphrase))
				if err != nil {
					log.Panicf("invalid private key passphrase: %s", name)
				}
			}
		}
	}
}

func GetGPGKey(name string) *Key {
	return gpgKeys[name]
}

// Ubuntu Archive Automatic Signing Key (2018) <ftpmaster@ubuntu.com>.
// Key ID: 871920D1991BC93C.
// Useful to validate InRelease files from live archive.
const ubuntuArchiveSignKey2018 = `
-----BEGIN PGP PUBLIC KEY BLOCK-----

mQINBFufwdoBEADv/Gxytx/LcSXYuM0MwKojbBye81s0G1nEx+lz6VAUpIUZnbkq
dXBHC+dwrGS/CeeLuAjPRLU8AoxE/jjvZVp8xFGEWHYdklqXGZ/gJfP5d3fIUBtZ
HZEJl8B8m9pMHf/AQQdsC+YzizSG5t5Mhnotw044LXtdEEkx2t6Jz0OGrh+5Ioxq
X7pZiq6Cv19BohaUioKMdp7ES6RYfN7ol6HSLFlrMXtVfh/ijpN9j3ZhVGVeRC8k
KHQsJ5PkIbmvxBiUh7SJmfZUx0IQhNMaDHXfdZAGNtnhzzNReb1FqNLSVkrS/Pns
AQzMhG1BDm2VOSF64jebKXffFqM5LXRQTeqTLsjUbbrqR6s/GCO8UF7jfUj6I7ta
LygmsHO/JD4jpKRC0gbpUBfaiJyLvuepx3kWoqL3sN0LhlMI80+fA7GTvoOx4tpq
VlzlE6TajYu+jfW3QpOFS5ewEMdL26hzxsZg/geZvTbArcP+OsJKRmhv4kNo6Ayd
yHQ/3ZV/f3X9mT3/SPLbJaumkgp3Yzd6t5PeBu+ZQk/mN5WNNuaihNEV7llb1Zhv
Y0Fxu9BVd/BNl0rzuxp3rIinB2TX2SCg7wE5xXkwXuQ/2eTDE0v0HlGntkuZjGow
DZkxHZQSxZVOzdZCRVaX/WEFLpKa2AQpw5RJrQ4oZ/OfifXyJzP27o03wQARAQAB
tEJVYnVudHUgQXJjaGl2ZSBBdXRvbWF0aWMgU2lnbmluZyBLZXkgKDIwMTgpIDxm
dHBtYXN0ZXJAdWJ1bnR1LmNvbT6JAjgEEwEKACIFAlufwdoCGwMGCwkIBwMCBhUI
AgkKCwQWAgMBAh4BAheAAAoJEIcZINGZG8k8LHMQAKS2cnxz/5WaoCOWArf5g6UH
beOCgc5DBm0hCuFDZWWv427aGei3CPuLw0DGLCXZdyc5dqE8mvjMlOmmAKKlj1uG
g3TYCbQWjWPeMnBPZbkFgkZoXJ7/6CB7bWRht1sHzpt1LTZ+SYDwOwJ68QRp7DRa
Zl9Y6QiUbeuhq2DUcTofVbBxbhrckN4ZteLvm+/nG9m/ciopc66LwRdkxqfJ32Cy
q+1TS5VaIJDG7DWziG+Kbu6qCDM4QNlg3LH7p14CrRxAbc4lvohRgsV4eQqsIcdF
kuVY5HPPj2K8TqpY6STe8Gh0aprG1RV8ZKay3KSMpnyV1fAKn4fM9byiLzQAovC0
LZ9MMMsrAS/45AvC3IEKSShjLFn1X1dRCiO6/7jmZEoZtAp53hkf8SMBsi78hVNr
BumZwfIdBA1v22+LY4xQK8q4XCoRcA9G+pvzU9YVW7cRnDZZGl0uwOw7z9PkQBF5
KFKjWDz4fCk+K6+YtGpovGKekGBb8I7EA6UpvPgqA/QdI0t1IBP0N06RQcs1fUaA
QEtz6DGy5zkRhR4pGSZn+dFET7PdAjEK84y7BdY4t+U1jcSIvBj0F2B7LwRL7xGp
SpIKi/ekAXLs117bvFHaCvmUYN7JVp1GMmVFxhIdx6CFm3fxG8QjNb5tere/YqK+
uOgcXny1UlwtCUzlrSaPiQIzBBABCgAdFiEEFT8cnvE5X78ANS6NC/uEfz8nL1sF
AlufxEMACgkQC/uEfz8nL1tuFw/9GgaeggvCn15QplABa86OReJARxnAxpaL223p
LkgAbBYAOT7PmTjwwHCqGeJZGLzAQsGLc6WkQDegewQCMWLp+1zOHmUBHbZPsz3E
76Ac381FAXhZBj8MLbcyOROsKYKZ9M/yGerMpVx4B8WNb5P+t9ttAwwAR/lNs5OS
3lpV4nkwIzvxA6Wnq0gWKBL/9rc7sL+qWeJDnQEkq1Z/dNBbgIWktDtqeIXFldgj
YOX+x1RN81beLVDtRLoOU0IkQsFGaOOb0o2x8/dmYM2cXuchNGYmdY2Z5jeLI1F0
dzCR+CRUEDFdr0cF94USgVGWyCoaHdABTRD5e/uIEySL0T9ym93RNBtoc9gPENFB
2ASMJgkMNINiV82alPjYYrbs+ZVHuLQIgd+qw/N6zwLtVDgo2Pc6FXZpqmSjRRmt
BRJuv+VnDBeAOstl0QloRm5gRBp/wgt93E1Ah+QJRVuMQFqz0nPZWTwfcGagmSEu
rWiKX8n2FFYkiLfyUW0335TN88Z99+gvQ+AySAFu8ReT/lQzAPRPNRLjpAk5e1Fu
MzQYoBJcYwP0sjAIO1AWmguPI1KLfnVnXnsT5JYMbG2DCLHI/OIvnpRq8v955glZ
5L9aq8bNnOwC2BK6MVUspbJRpGLQ29hbeH8jnRPOPQ+Sbwa2C8/ZSoBa/L6JGl5R
DaOLQ1w=
=+8/z
-----END PGP PUBLIC KEY BLOCK-----
`

// Test-purpose RSA 2048 bits signing key-pairs without a passphrase.
// Key ID: 854BAF1AA9D76600. User: "foo-bar <foo@bar>".
const testPublicKeyData = `
-----BEGIN PGP PUBLIC KEY BLOCK-----

mQENBGVs8P4BCADPh/fNnw2AI1JCYf+3p4jkcFQPLVsUkoTZk8OXjCxy+UP9Jd2m
xnxat7a0JEJZa0aWCmtlSL1XR+kFKBrd7Ry5jOHYjuDKx4kTmDUbezPnjoZIGDNX
j5cdNuMLpOINZweNNWDKRdRvhj5QX89/DYwPrLkNFwwjXjlj5tjU6RUkROYJBGPe
G2ns2cZtVbYMh3FDU9YRfp/hUqGVf+UFRyUw+mo1TUlk5F7fnfwEQmsppDHvfTNJ
yjEMZD7nReTEeMy12GV2wysOwWMPEb2PSE/+Od7AKn5dFA7w3kyLCzAxYp6o7IE/
+RY8YzAJe6GmLwhTWtylMV1xteQhZkEe/QGXABEBAAG0EWZvby1iYXIgPGZvb0Bi
YXI+iQFOBBMBCgA4FiEEDp0LAdsRnT9gfhU5hUuvGqnXZgAFAmVs8P4CGwMFCwkI
BwIGFQoJCAsCBBYCAwECHgECF4AACgkQhUuvGqnXZgCHZAf/b/rkMz2UY42LhuvJ
xDW7KbdBI+UgFp2k2tg2SkLM27GdcztpcNn/RE9U1vc8uCI05MbMhKQ+oq4RmO6i
QbCPPGy1Mgf61Fku0JTZGEKg+4DKNmnVkSpiOc03z3G2Gyi2m9G2u+HdJhXHumej
7NXkQvVFxXzDnzntbnmkM0fMfO+wdP5/EFjJbHC47yAAds/yspfk5qIHu6PHrTVB
+wJGwOJdwJ1+2zis5ONE8NexfSrDzjGJoKAFtlMwNNDZ39JlkguMB0M5SxoGRXxQ
ZE4DhPntUIW0qsE6ChmmjssjSDeg75rwgc+hjNDunKQhKNpjVVFGF4uceV5EQ084
F4nA5w==
=ZXap
-----END PGP PUBLIC KEY BLOCK-----
`
const testPrivateKeyData = `
-----BEGIN PGP PRIVATE KEY BLOCK-----

lQOYBGVs8P4BCADPh/fNnw2AI1JCYf+3p4jkcFQPLVsUkoTZk8OXjCxy+UP9Jd2m
xnxat7a0JEJZa0aWCmtlSL1XR+kFKBrd7Ry5jOHYjuDKx4kTmDUbezPnjoZIGDNX
j5cdNuMLpOINZweNNWDKRdRvhj5QX89/DYwPrLkNFwwjXjlj5tjU6RUkROYJBGPe
G2ns2cZtVbYMh3FDU9YRfp/hUqGVf+UFRyUw+mo1TUlk5F7fnfwEQmsppDHvfTNJ
yjEMZD7nReTEeMy12GV2wysOwWMPEb2PSE/+Od7AKn5dFA7w3kyLCzAxYp6o7IE/
+RY8YzAJe6GmLwhTWtylMV1xteQhZkEe/QGXABEBAAEAB/4jvxdbdyiTqEHchlXO
NBDbzE9mV9km53/znESl/3KOkUn5OkL+HZVA6QES8WXuUhCT+pJ6HTfj51KHXVuX
W2bFvTMPorispQcC9YY8SBHuMjoGBAkf7W9JjHE6SbnYNiVyWL3lyXZoiVaFcKNk
jphQAN/VFeG029+FyjcSIV3PY7FWI4Q1dyqyf78iWa6I400cmyGFvZDSps/oo3sT
0xcjdLL5AaXyR0FtZoSrltioYzp4cnYDI2ES9PT7uR6MQ7AwUamUQ/7dUR6zSi1o
NbHVOYItsZEsY8N/1vUxW+Ps0bbgZd9ob6n+1beQIeSMhJiW0g2NiqlZXo8GELNp
LNOBBADl+tu0iX0DCTJ5fnDeiWgMv+sPA9pcACKhnxDuOXMJjV/gGY2XtKzP0o68
y8N5Nry0UG3wHMlgqp5qY8ZkXfH3zMmIezG5C6HZQ7A44wem3iBYj8Z1bjpT8AW7
rFi+1iBDmZ4whHzsxLp8XB/cugAh/g3bo6rJl2bCaQPnpsSygQQA5wLnFL8pnj4M
kNzefp/ZFGTstB7AC1Dfkja9QTfimZpJZj/5XXyewAgmqQt9uersmLHfXhS3sgrk
kko74ZEZY5PCInsbcvUkgRxgw/JnjWdHLVUOMMd12RVQU9BOVf2kN8sEWCQbqzsM
H9IEtFjXXyyubmb4euI25xs1ptxk+BcD/j1J5bu6RZfP2IfEeBPu4w8zK5WOioLY
dia8kvzScIRvREB6DbYCifirx0gSuZSCyo+zm/KfZCof89ihOZ4e3OAWQDqajfQH
AGoXJCN9LRJsGe/x79LHuOx71x1MbTTvOUlYJTD9+cHzWRzKHb2ecFL6jaJb4OhY
RP4t194OXMHdQ2q0EWZvby1iYXIgPGZvb0BiYXI+iQFOBBMBCgA4FiEEDp0LAdsR
nT9gfhU5hUuvGqnXZgAFAmVs8P4CGwMFCwkIBwIGFQoJCAsCBBYCAwECHgECF4AA
CgkQhUuvGqnXZgCHZAf/b/rkMz2UY42LhuvJxDW7KbdBI+UgFp2k2tg2SkLM27Gd
cztpcNn/RE9U1vc8uCI05MbMhKQ+oq4RmO6iQbCPPGy1Mgf61Fku0JTZGEKg+4DK
NmnVkSpiOc03z3G2Gyi2m9G2u+HdJhXHumej7NXkQvVFxXzDnzntbnmkM0fMfO+w
dP5/EFjJbHC47yAAds/yspfk5qIHu6PHrTVB+wJGwOJdwJ1+2zis5ONE8NexfSrD
zjGJoKAFtlMwNNDZ39JlkguMB0M5SxoGRXxQZE4DhPntUIW0qsE6ChmmjssjSDeg
75rwgc+hjNDunKQhKNpjVVFGF4uceV5EQ084F4nA5w==
=VBWI
-----END PGP PRIVATE KEY BLOCK-----
`
