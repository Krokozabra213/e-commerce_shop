package infrajwt

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
)

func ParseRSAPublicKey(pemStr string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("decode PEM block: no valid PEM block found")
	}

	var pubKey *rsa.PublicKey
	var parseErr error

	if keyInterface, pkixErr := x509.ParsePKIXPublicKey(block.Bytes); pkixErr == nil {
		if pk, ok := keyInterface.(*rsa.PublicKey); ok {
			pubKey = pk
		} else {
			parseErr = fmt.Errorf("key is not an RSA public key (got %T)", keyInterface)
		}
	} else {
		parseErr = pkixErr
	}

	if pubKey == nil {
		if pk, pkcsErr := x509.ParsePKCS1PublicKey(block.Bytes); pkcsErr == nil {
			pubKey = pk
		} else if parseErr == nil {
			parseErr = pkcsErr
		}
	}

	if pubKey == nil {
		if parseErr == nil {
			return nil, errors.New("parse public key: unsupported format or invalid key")
		}
		return nil, fmt.Errorf("parse public key: %w", parseErr)
	}

	if pubKey.N.BitLen() < 2048 {
		return nil, errors.New("public key modulus too small (must be at least 2048 bits)")
	}

	if pubKey.E <= 1 || pubKey.E == 1 {
		return nil, errors.New("invalid RSA public exponent")
	}

	return pubKey, nil
}
