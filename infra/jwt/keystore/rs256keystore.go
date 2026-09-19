package jwtkeystore

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"sync"

	"github.com/golang-jwt/jwt/v5"
)

const (
	jwkKeyTypeRSA = "RSA" // RFC 7518 §6.1
	jwkUseSig     = "sig" // RFC 7517 §4.2
)

var ErrEmptyPrivateKey = errors.New("empty private key")

type RS256KeyStore struct {
	mu         sync.RWMutex
	privateKey *rsa.PrivateKey
	kid        string
}

func NewRS256KeyStore(privateKey *rsa.PrivateKey, kid string) (*RS256KeyStore, error) {
	if privateKey == nil {
		return nil, ErrEmptyPrivateKey
	}
	return &RS256KeyStore{
		privateKey: privateKey,
		kid:        kid,
	}, nil
}

func (ks *RS256KeyStore) PrivateKey() *rsa.PrivateKey {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	return ks.privateKey
}

func (ks *RS256KeyStore) PublicKey() *rsa.PublicKey {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	return &ks.privateKey.PublicKey
}

func (ks *RS256KeyStore) GetPublicKeyPEM() (string, error) {
	pubKey := ks.PublicKey()
	asn1Bytes, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		return "", fmt.Errorf("marshal public key to asn1: %w", err)
	}

	pemBlock := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: asn1Bytes,
	}

	return string(pem.EncodeToMemory(pemBlock)), nil
}

func (ks *RS256KeyStore) KID() string {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	return ks.kid
}

type JWKSResponse struct {
	Keys []JWK `json:"keys"`
}

type JWK struct {
	KTY string `json:"kty"`
	KID string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func (ks *RS256KeyStore) JWKS() JWKSResponse {
	ks.mu.RLock()
	defer ks.mu.RUnlock()

	pub := &ks.privateKey.PublicKey

	return JWKSResponse{
		Keys: []JWK{
			{
				KTY: jwkKeyTypeRSA,
				KID: ks.kid,
				Use: jwkUseSig,
				Alg: jwt.SigningMethodRS256.Alg(),
				N:   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
				E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
			},
		},
	}
}

func (ks *RS256KeyStore) Rotate(newPrivateKey *rsa.PrivateKey, newKID string) error {
	if newPrivateKey == nil {
		return ErrEmptyPrivateKey
	}
	ks.mu.Lock()
	defer ks.mu.Unlock()
	ks.privateKey = newPrivateKey
	ks.kid = newKID
	return nil
}
