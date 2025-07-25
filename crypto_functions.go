// crypto_functions.go - Crypto module implementation for Hype
package main

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yuin/gopher-lua"
)

// JWK represents a JSON Web Key
type JWK struct {
	Kty string `json:"kty"`           // Key Type
	Alg string `json:"alg,omitempty"` // Algorithm
	Use string `json:"use,omitempty"` // Public Key Use
	Kid string `json:"kid,omitempty"` // Key ID
	
	// RSA keys
	N string `json:"n,omitempty"` // Modulus
	E string `json:"e,omitempty"` // Exponent
	D string `json:"d,omitempty"` // Private Exponent
	P string `json:"p,omitempty"` // First Prime Factor
	Q string `json:"q,omitempty"` // Second Prime Factor
	Dp string `json:"dp,omitempty"` // First Factor CRT Exponent
	Dq string `json:"dq,omitempty"` // Second Factor CRT Exponent
	Qi string `json:"qi,omitempty"` // First CRT Coefficient
	
	// ECDSA keys
	Crv string `json:"crv,omitempty"` // Curve
	X   string `json:"x,omitempty"`   // X Coordinate
	Y   string `json:"y,omitempty"`   // Y Coordinate
	
	// EdDSA keys (same as ECDSA for Ed25519)
}

// registerCryptoModule adds crypto functionality to Lua
func registerCryptoModule(L *lua.LState) {
	L.PreloadModule("crypto", func(L *lua.LState) int {
		cryptoModule := L.NewTable()
		
		L.SetField(cryptoModule, "generate_jwk", L.NewFunction(cryptoGenerateJWK))
		L.SetField(cryptoModule, "sign", L.NewFunction(cryptoSign))
		L.SetField(cryptoModule, "verify", L.NewFunction(cryptoVerify))
		L.SetField(cryptoModule, "jwk_to_public", L.NewFunction(cryptoJWKToPublic))
		L.SetField(cryptoModule, "jwk_thumbprint", L.NewFunction(cryptoJWKThumbprint))
		L.SetField(cryptoModule, "jwk_to_json", L.NewFunction(cryptoJWKToJSON))
		L.SetField(cryptoModule, "jwk_from_json", L.NewFunction(cryptoJWKFromJSON))
		
		// Hashing functions
		L.SetField(cryptoModule, "sha256", L.NewFunction(cryptoSHA256))
		L.SetField(cryptoModule, "sha384", L.NewFunction(cryptoSHA384))
		L.SetField(cryptoModule, "sha512", L.NewFunction(cryptoSHA512))
		L.SetField(cryptoModule, "hash", L.NewFunction(cryptoHash))
		L.SetField(cryptoModule, "deep_hash", L.NewFunction(cryptoDeepHash))
		L.SetField(cryptoModule, "digest", L.NewFunction(cryptoDigest))
		
		// Base64 functions
		L.SetField(cryptoModule, "base64urlDecode", L.NewFunction(cryptoBase64URLDecode))
		
		L.Push(cryptoModule)
		return 1
	})
}

// cryptoGenerateJWK generates a new JWK keypair
func cryptoGenerateJWK(L *lua.LState) int {
	algorithm := L.ToString(1)
	if algorithm == "" {
		algorithm = "RS256" // default
	}
	
	// Optional second parameter for key size (RSA only)
	keySize := 0
	if L.GetTop() >= 2 {
		keySize = L.ToInt(2)
	}
	
	var jwk *JWK
	var err error
	
	switch algorithm {
	case "RS256", "RS384", "RS512", "PS256", "PS384", "PS512":
		jwk, err = generateRSAJWK(algorithm, keySize)
	case "ES256", "ES384", "ES512":
		jwk, err = generateECDSAJWK(algorithm)
	case "EdDSA":
		jwk, err = generateEd25519JWK()
	default:
		L.Push(lua.LNil)
		L.Push(lua.LString("unsupported algorithm: " + algorithm))
		return 2
	}
	
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("failed to generate key: " + err.Error()))
		return 2
	}
	
	// Convert JWK to Lua table
	jwkTable := jwkToLuaTable(L, jwk)
	L.Push(jwkTable)
	return 1
}

// generateRSAJWK creates an RSA JWK
func generateRSAJWK(algorithm string, requestedKeySize int) (*JWK, error) {
	keySize := 2048  // default
	
	// If a specific key size was requested, validate and use it
	if requestedKeySize > 0 {
		if requestedKeySize == 2048 || requestedKeySize == 3072 || requestedKeySize == 4096 {
			keySize = requestedKeySize
		} else {
			return nil, fmt.Errorf("unsupported RSA key size: %d (supported: 2048, 3072, 4096)", requestedKeySize)
		}
	} else {
		// Use default key sizes based on algorithm
		if algorithm == "RS384" || algorithm == "RS512" || algorithm == "PS384" || algorithm == "PS512" {
			keySize = 3072
		}
	}
	
	privateKey, err := rsa.GenerateKey(rand.Reader, keySize)
	if err != nil {
		return nil, err
	}
	
	jwk := &JWK{
		Kty: "RSA",
		Alg: algorithm,
		Use: "sig",
		Kid: fmt.Sprintf("%d", time.Now().Unix()),
		N:   base64.RawURLEncoding.EncodeToString(privateKey.N.Bytes()),
		E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privateKey.E)).Bytes()),
		D:   base64.RawURLEncoding.EncodeToString(privateKey.D.Bytes()),
		P:   base64.RawURLEncoding.EncodeToString(privateKey.Primes[0].Bytes()),
		Q:   base64.RawURLEncoding.EncodeToString(privateKey.Primes[1].Bytes()),
	}
	
	// Calculate CRT values
	dp := new(big.Int).Mod(privateKey.D, new(big.Int).Sub(privateKey.Primes[0], big.NewInt(1)))
	dq := new(big.Int).Mod(privateKey.D, new(big.Int).Sub(privateKey.Primes[1], big.NewInt(1)))
	qi := new(big.Int).ModInverse(privateKey.Primes[1], privateKey.Primes[0])
	
	jwk.Dp = base64.RawURLEncoding.EncodeToString(dp.Bytes())
	jwk.Dq = base64.RawURLEncoding.EncodeToString(dq.Bytes())
	jwk.Qi = base64.RawURLEncoding.EncodeToString(qi.Bytes())
	
	return jwk, nil
}

// generateECDSAJWK creates an ECDSA JWK
func generateECDSAJWK(algorithm string) (*JWK, error) {
	var curve elliptic.Curve
	var crvName string
	
	switch algorithm {
	case "ES256":
		curve = elliptic.P256()
		crvName = "P-256"
	case "ES384":
		curve = elliptic.P384()
		crvName = "P-384"
	case "ES512":
		curve = elliptic.P521()
		crvName = "P-521"
	default:
		return nil, fmt.Errorf("unsupported ECDSA algorithm: %s", algorithm)
	}
	
	privateKey, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		return nil, err
	}
	
	// Get curve size for padding
	keySize := (curve.Params().BitSize + 7) / 8
	
	xBytes := privateKey.X.Bytes()
	yBytes := privateKey.Y.Bytes()
	dBytes := privateKey.D.Bytes()
	
	// Pad to correct length
	if len(xBytes) < keySize {
		padded := make([]byte, keySize)
		copy(padded[keySize-len(xBytes):], xBytes)
		xBytes = padded
	}
	if len(yBytes) < keySize {
		padded := make([]byte, keySize)
		copy(padded[keySize-len(yBytes):], yBytes)
		yBytes = padded
	}
	if len(dBytes) < keySize {
		padded := make([]byte, keySize)
		copy(padded[keySize-len(dBytes):], dBytes)
		dBytes = padded
	}
	
	jwk := &JWK{
		Kty: "EC",
		Alg: algorithm,
		Use: "sig",
		Kid: fmt.Sprintf("%d", time.Now().Unix()),
		Crv: crvName,
		X:   base64.RawURLEncoding.EncodeToString(xBytes),
		Y:   base64.RawURLEncoding.EncodeToString(yBytes),
		D:   base64.RawURLEncoding.EncodeToString(dBytes),
	}
	
	return jwk, nil
}

// generateEd25519JWK creates an Ed25519 JWK
func generateEd25519JWK() (*JWK, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	
	jwk := &JWK{
		Kty: "OKP",
		Alg: "EdDSA",
		Use: "sig",
		Kid: fmt.Sprintf("%d", time.Now().Unix()),
		Crv: "Ed25519",
		X:   base64.RawURLEncoding.EncodeToString(publicKey),
		D:   base64.RawURLEncoding.EncodeToString(privateKey[:32]), // Ed25519 private key is 32 bytes
	}
	
	return jwk, nil
}

// cryptoSign signs data with a JWK
func cryptoSign(L *lua.LState) int {
	// Check if first argument is algorithm string (new format) or jwk table (old format)
	var algorithm string
	var jwkTable *lua.LTable
	var data string
	
	if L.GetTop() >= 3 && L.Get(1).Type() == lua.LTString {
		// New format: sign(algorithm, data, jwk)
		algorithm = L.ToString(1)
		data = L.ToString(2)
		jwkTable = L.ToTable(3)
	} else {
		// Old format: sign(jwk, data) - for backward compatibility
		jwkTable = L.ToTable(1)
		data = L.ToString(2)
	}
	
	if jwkTable == nil || data == "" {
		L.Push(lua.LNil)
		L.Push(lua.LString("missing jwk or data"))
		return 2
	}
	
	jwk, err := luaTableToJWK(jwkTable)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("invalid jwk: " + err.Error()))
		return 2
	}
	
	// Override algorithm if specified
	if algorithm != "" {
		// Map common algorithm names to JWK algorithm names
		switch strings.ToLower(algorithm) {
		case "rsa-pss":
			// Default to PS256 if no algorithm specified in JWK
			if jwk.Alg == "" || !strings.HasPrefix(jwk.Alg, "PS") {
				jwk.Alg = "PS256"
			}
		default:
			jwk.Alg = algorithm
		}
	}
	
	signature, err := signWithJWK(jwk, []byte(data))
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("signing failed: " + err.Error()))
		return 2
	}
	
	L.Push(lua.LString(base64.RawURLEncoding.EncodeToString(signature)))
	return 1
}

// cryptoVerify verifies a signature with a JWK
func cryptoVerify(L *lua.LState) int {
	jwkTable := L.ToTable(1)
	data := L.ToString(2)
	signatureB64 := L.ToString(3)
	
	if jwkTable == nil || data == "" || signatureB64 == "" {
		L.Push(lua.LBool(false))
		L.Push(lua.LString("missing jwk, data, or signature"))
		return 2
	}
	
	jwk, err := luaTableToJWK(jwkTable)
	if err != nil {
		L.Push(lua.LBool(false))
		L.Push(lua.LString("invalid jwk: " + err.Error()))
		return 2
	}
	
	signature, err := base64.RawURLEncoding.DecodeString(signatureB64)
	if err != nil {
		L.Push(lua.LBool(false))
		L.Push(lua.LString("invalid signature encoding: " + err.Error()))
		return 2
	}
	
	valid, err := verifyWithJWK(jwk, []byte(data), signature)
	if err != nil {
		L.Push(lua.LBool(false))
		L.Push(lua.LString("verification failed: " + err.Error()))
		return 2
	}
	
	L.Push(lua.LBool(valid))
	return 1
}

// signWithJWK signs data using a JWK
func signWithJWK(jwk *JWK, data []byte) ([]byte, error) {
	switch jwk.Kty {
	case "RSA":
		return signRSA(jwk, data)
	case "EC":
		return signECDSA(jwk, data)
	case "OKP":
		if jwk.Crv == "Ed25519" {
			return signEd25519(jwk, data)
		}
		return nil, fmt.Errorf("unsupported OKP curve: %s", jwk.Crv)
	default:
		return nil, fmt.Errorf("unsupported key type: %s", jwk.Kty)
	}
}

// verifyWithJWK verifies a signature using a JWK
func verifyWithJWK(jwk *JWK, data []byte, signature []byte) (bool, error) {
	switch jwk.Kty {
	case "RSA":
		return verifyRSA(jwk, data, signature)
	case "EC":
		return verifyECDSA(jwk, data, signature)
	case "OKP":
		if jwk.Crv == "Ed25519" {
			return verifyEd25519(jwk, data, signature)
		}
		return false, fmt.Errorf("unsupported OKP curve: %s", jwk.Crv)
	default:
		return false, fmt.Errorf("unsupported key type: %s", jwk.Kty)
	}
}

// signRSA signs data with RSA
func signRSA(jwk *JWK, data []byte) ([]byte, error) {
	privateKey, err := jwkToRSAPrivateKey(jwk)
	if err != nil {
		return nil, err
	}
	
	var hash crypto.Hash
	switch jwk.Alg {
	case "RS256", "PS256":
		hash = crypto.SHA256
	case "RS384", "PS384":
		hash = crypto.SHA384
	case "RS512", "PS512":
		hash = crypto.SHA512
	default:
		return nil, fmt.Errorf("unsupported RSA algorithm: %s", jwk.Alg)
	}
	
	hasher := hash.New()
	hasher.Write(data)
	hashed := hasher.Sum(nil)
	
	// Use PSS for PS algorithms
	switch jwk.Alg {
	case "PS256", "PS384", "PS512":
		opts := &rsa.PSSOptions{
			SaltLength: rsa.PSSSaltLengthEqualsHash,
			Hash:       hash,
		}
		return rsa.SignPSS(rand.Reader, privateKey, hash, hashed, opts)
	default:
		return rsa.SignPKCS1v15(rand.Reader, privateKey, hash, hashed)
	}
}

// verifyRSA verifies an RSA signature
func verifyRSA(jwk *JWK, data []byte, signature []byte) (bool, error) {
	publicKey, err := jwkToRSAPublicKey(jwk)
	if err != nil {
		return false, err
	}
	
	var hash crypto.Hash
	switch jwk.Alg {
	case "RS256", "PS256":
		hash = crypto.SHA256
	case "RS384", "PS384":
		hash = crypto.SHA384
	case "RS512", "PS512":
		hash = crypto.SHA512
	default:
		return false, fmt.Errorf("unsupported RSA algorithm: %s", jwk.Alg)
	}
	
	hasher := hash.New()
	hasher.Write(data)
	hashed := hasher.Sum(nil)
	
	// Use PSS for PS algorithms
	switch jwk.Alg {
	case "PS256", "PS384", "PS512":
		opts := &rsa.PSSOptions{
			SaltLength: rsa.PSSSaltLengthEqualsHash,
			Hash:       hash,
		}
		err = rsa.VerifyPSS(publicKey, hash, hashed, signature, opts)
	default:
		err = rsa.VerifyPKCS1v15(publicKey, hash, hashed, signature)
	}
	return err == nil, nil
}

// signECDSA signs data with ECDSA
func signECDSA(jwk *JWK, data []byte) ([]byte, error) {
	privateKey, err := jwkToECDSAPrivateKey(jwk)
	if err != nil {
		return nil, err
	}
	
	var hasher crypto.Hash
	switch jwk.Alg {
	case "ES256":
		hasher = crypto.SHA256
	case "ES384":
		hasher = crypto.SHA384
	case "ES512":
		hasher = crypto.SHA512
	default:
		return nil, fmt.Errorf("unsupported ECDSA algorithm: %s", jwk.Alg)
	}
	
	hash := hasher.New()
	hash.Write(data)
	hashed := hash.Sum(nil)
	
	r, s, err := ecdsa.Sign(rand.Reader, privateKey, hashed)
	if err != nil {
		return nil, err
	}
	
	// Get curve size for signature formatting
	keySize := (privateKey.Curve.Params().BitSize + 7) / 8
	
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	
	// Pad to correct length
	signature := make([]byte, 2*keySize)
	copy(signature[keySize-len(rBytes):keySize], rBytes)
	copy(signature[2*keySize-len(sBytes):], sBytes)
	
	return signature, nil
}

// verifyECDSA verifies an ECDSA signature
func verifyECDSA(jwk *JWK, data []byte, signature []byte) (bool, error) {
	publicKey, err := jwkToECDSAPublicKey(jwk)
	if err != nil {
		return false, err
	}
	
	var hasher crypto.Hash
	switch jwk.Alg {
	case "ES256":
		hasher = crypto.SHA256
	case "ES384":
		hasher = crypto.SHA384
	case "ES512":
		hasher = crypto.SHA512
	default:
		return false, fmt.Errorf("unsupported ECDSA algorithm: %s", jwk.Alg)
	}
	
	hash := hasher.New()
	hash.Write(data)
	hashed := hash.Sum(nil)
	
	// Parse signature (r, s values)
	keySize := len(signature) / 2
	r := new(big.Int).SetBytes(signature[:keySize])
	s := new(big.Int).SetBytes(signature[keySize:])
	
	return ecdsa.Verify(publicKey, hashed, r, s), nil
}

// signEd25519 signs data with Ed25519
func signEd25519(jwk *JWK, data []byte) ([]byte, error) {
	privateKey, err := jwkToEd25519PrivateKey(jwk)
	if err != nil {
		return nil, err
	}
	
	return ed25519.Sign(privateKey, data), nil
}

// verifyEd25519 verifies an Ed25519 signature
func verifyEd25519(jwk *JWK, data []byte, signature []byte) (bool, error) {
	publicKey, err := jwkToEd25519PublicKey(jwk)
	if err != nil {
		return false, err
	}
	
	return ed25519.Verify(publicKey, data, signature), nil
}

// Helper functions for JWK conversion
func jwkToRSAPrivateKey(jwk *JWK) (*rsa.PrivateKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, err
	}
	dBytes, err := base64.RawURLEncoding.DecodeString(jwk.D)
	if err != nil {
		return nil, err
	}
	pBytes, err := base64.RawURLEncoding.DecodeString(jwk.P)
	if err != nil {
		return nil, err
	}
	qBytes, err := base64.RawURLEncoding.DecodeString(jwk.Q)
	if err != nil {
		return nil, err
	}
	
	return &rsa.PrivateKey{
		PublicKey: rsa.PublicKey{
			N: new(big.Int).SetBytes(nBytes),
			E: int(new(big.Int).SetBytes(eBytes).Int64()),
		},
		D:      new(big.Int).SetBytes(dBytes),
		Primes: []*big.Int{new(big.Int).SetBytes(pBytes), new(big.Int).SetBytes(qBytes)},
	}, nil
}

func jwkToRSAPublicKey(jwk *JWK) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, err
	}
	
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: int(new(big.Int).SetBytes(eBytes).Int64()),
	}, nil
}

func jwkToECDSAPrivateKey(jwk *JWK) (*ecdsa.PrivateKey, error) {
	var curve elliptic.Curve
	switch jwk.Crv {
	case "P-256":
		curve = elliptic.P256()
	case "P-384":
		curve = elliptic.P384()
	case "P-521":
		curve = elliptic.P521()
	default:
		return nil, fmt.Errorf("unsupported curve: %s", jwk.Crv)
	}
	
	xBytes, err := base64.RawURLEncoding.DecodeString(jwk.X)
	if err != nil {
		return nil, err
	}
	yBytes, err := base64.RawURLEncoding.DecodeString(jwk.Y)
	if err != nil {
		return nil, err
	}
	dBytes, err := base64.RawURLEncoding.DecodeString(jwk.D)
	if err != nil {
		return nil, err
	}
	
	return &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{
			Curve: curve,
			X:     new(big.Int).SetBytes(xBytes),
			Y:     new(big.Int).SetBytes(yBytes),
		},
		D: new(big.Int).SetBytes(dBytes),
	}, nil
}

func jwkToECDSAPublicKey(jwk *JWK) (*ecdsa.PublicKey, error) {
	var curve elliptic.Curve
	switch jwk.Crv {
	case "P-256":
		curve = elliptic.P256()
	case "P-384":
		curve = elliptic.P384()
	case "P-521":
		curve = elliptic.P521()
	default:
		return nil, fmt.Errorf("unsupported curve: %s", jwk.Crv)
	}
	
	xBytes, err := base64.RawURLEncoding.DecodeString(jwk.X)
	if err != nil {
		return nil, err
	}
	yBytes, err := base64.RawURLEncoding.DecodeString(jwk.Y)
	if err != nil {
		return nil, err
	}
	
	return &ecdsa.PublicKey{
		Curve: curve,
		X:     new(big.Int).SetBytes(xBytes),
		Y:     new(big.Int).SetBytes(yBytes),
	}, nil
}

func jwkToEd25519PrivateKey(jwk *JWK) (ed25519.PrivateKey, error) {
	xBytes, err := base64.RawURLEncoding.DecodeString(jwk.X)
	if err != nil {
		return nil, err
	}
	dBytes, err := base64.RawURLEncoding.DecodeString(jwk.D)
	if err != nil {
		return nil, err
	}
	
	// Ed25519 private key is 64 bytes: 32 bytes private + 32 bytes public
	privateKey := make(ed25519.PrivateKey, ed25519.PrivateKeySize)
	copy(privateKey[:32], dBytes)
	copy(privateKey[32:], xBytes)
	
	return privateKey, nil
}

func jwkToEd25519PublicKey(jwk *JWK) (ed25519.PublicKey, error) {
	xBytes, err := base64.RawURLEncoding.DecodeString(jwk.X)
	if err != nil {
		return nil, err
	}
	
	return ed25519.PublicKey(xBytes), nil
}

// Helper functions for Lua table conversion
func jwkToLuaTable(L *lua.LState, jwk *JWK) *lua.LTable {
	table := L.NewTable()
	
	L.SetField(table, "kty", lua.LString(jwk.Kty))
	if jwk.Alg != "" {
		L.SetField(table, "alg", lua.LString(jwk.Alg))
	}
	if jwk.Use != "" {
		L.SetField(table, "use", lua.LString(jwk.Use))
	}
	if jwk.Kid != "" {
		L.SetField(table, "kid", lua.LString(jwk.Kid))
	}
	
	// RSA fields
	if jwk.N != "" {
		L.SetField(table, "n", lua.LString(jwk.N))
	}
	if jwk.E != "" {
		L.SetField(table, "e", lua.LString(jwk.E))
	}
	if jwk.D != "" {
		L.SetField(table, "d", lua.LString(jwk.D))
	}
	if jwk.P != "" {
		L.SetField(table, "p", lua.LString(jwk.P))
	}
	if jwk.Q != "" {
		L.SetField(table, "q", lua.LString(jwk.Q))
	}
	if jwk.Dp != "" {
		L.SetField(table, "dp", lua.LString(jwk.Dp))
	}
	if jwk.Dq != "" {
		L.SetField(table, "dq", lua.LString(jwk.Dq))
	}
	if jwk.Qi != "" {
		L.SetField(table, "qi", lua.LString(jwk.Qi))
	}
	
	// ECDSA/EdDSA fields
	if jwk.Crv != "" {
		L.SetField(table, "crv", lua.LString(jwk.Crv))
	}
	if jwk.X != "" {
		L.SetField(table, "x", lua.LString(jwk.X))
	}
	if jwk.Y != "" {
		L.SetField(table, "y", lua.LString(jwk.Y))
	}
	
	return table
}

func luaTableToJWK(table *lua.LTable) (*JWK, error) {
	jwk := &JWK{}
	
	table.ForEach(func(key, value lua.LValue) {
		keyStr := key.String()
		valueStr := value.String()
		
		switch keyStr {
		case "kty":
			jwk.Kty = valueStr
		case "alg":
			jwk.Alg = valueStr
		case "use":
			jwk.Use = valueStr
		case "kid":
			jwk.Kid = valueStr
		case "n":
			jwk.N = valueStr
		case "e":
			jwk.E = valueStr
		case "d":
			jwk.D = valueStr
		case "p":
			jwk.P = valueStr
		case "q":
			jwk.Q = valueStr
		case "dp":
			jwk.Dp = valueStr
		case "dq":
			jwk.Dq = valueStr
		case "qi":
			jwk.Qi = valueStr
		case "crv":
			jwk.Crv = valueStr
		case "x":
			jwk.X = valueStr
		case "y":
			jwk.Y = valueStr
		}
	})
	
	if jwk.Kty == "" {
		return nil, fmt.Errorf("missing required field: kty")
	}
	
	return jwk, nil
}

// cryptoJWKToPublic extracts public key from JWK
func cryptoJWKToPublic(L *lua.LState) int {
	jwkTable := L.ToTable(1)
	if jwkTable == nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("missing jwk"))
		return 2
	}
	
	jwk, err := luaTableToJWK(jwkTable)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("invalid jwk: " + err.Error()))
		return 2
	}
	
	publicJWK := &JWK{
		Kty: jwk.Kty,
		Alg: jwk.Alg,
		Use: jwk.Use,
		Kid: jwk.Kid,
		N:   jwk.N,
		E:   jwk.E,
		Crv: jwk.Crv,
		X:   jwk.X,
		Y:   jwk.Y,
	}
	
	publicTable := jwkToLuaTable(L, publicJWK)
	L.Push(publicTable)
	return 1
}

// cryptoJWKThumbprint generates a thumbprint for a JWK
func cryptoJWKThumbprint(L *lua.LState) int {
	jwkTable := L.ToTable(1)
	if jwkTable == nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("missing jwk"))
		return 2
	}
	
	jwk, err := luaTableToJWK(jwkTable)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("invalid jwk: " + err.Error()))
		return 2
	}
	
	// Create canonical JSON for thumbprint (RFC 7638)
	var canonical map[string]interface{}
	switch jwk.Kty {
	case "RSA":
		canonical = map[string]interface{}{
			"e":   jwk.E,
			"kty": jwk.Kty,
			"n":   jwk.N,
		}
	case "EC":
		canonical = map[string]interface{}{
			"crv": jwk.Crv,
			"kty": jwk.Kty,
			"x":   jwk.X,
			"y":   jwk.Y,
		}
	case "OKP":
		canonical = map[string]interface{}{
			"crv": jwk.Crv,
			"kty": jwk.Kty,
			"x":   jwk.X,
		}
	default:
		L.Push(lua.LNil)
		L.Push(lua.LString("unsupported key type for thumbprint: " + jwk.Kty))
		return 2
	}
	
	canonicalJSON, err := json.Marshal(canonical)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("failed to create canonical JSON: " + err.Error()))
		return 2
	}
	
	hash := sha256.Sum256(canonicalJSON)
	thumbprint := base64.RawURLEncoding.EncodeToString(hash[:])
	
	L.Push(lua.LString(thumbprint))
	return 1
}

// cryptoJWKToJSON converts JWK to JSON string
func cryptoJWKToJSON(L *lua.LState) int {
	jwkTable := L.ToTable(1)
	if jwkTable == nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("missing jwk"))
		return 2
	}
	
	jwk, err := luaTableToJWK(jwkTable)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("invalid jwk: " + err.Error()))
		return 2
	}
	
	jsonData, err := json.Marshal(jwk)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("failed to marshal JSON: " + err.Error()))
		return 2
	}
	
	L.Push(lua.LString(string(jsonData)))
	return 1
}

// cryptoJWKFromJSON parses JWK from JSON string
func cryptoJWKFromJSON(L *lua.LState) int {
	jsonStr := L.ToString(1)
	if jsonStr == "" {
		L.Push(lua.LNil)
		L.Push(lua.LString("missing json string"))
		return 2
	}
	
	var jwk JWK
	if err := json.Unmarshal([]byte(jsonStr), &jwk); err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("invalid JSON: " + err.Error()))
		return 2
	}
	
	jwkTable := jwkToLuaTable(L, &jwk)
	L.Push(jwkTable)
	return 1
}

// cryptoSHA256 computes SHA-256 hash of input
func cryptoSHA256(L *lua.LState) int {
	data := L.ToString(1)
	hash := sha256.Sum256([]byte(data))
	L.Push(lua.LString(hex.EncodeToString(hash[:])))
	return 1
}

// cryptoSHA384 computes SHA-384 hash of input
func cryptoSHA384(L *lua.LState) int {
	data := L.ToString(1)
	hash := sha512.Sum384([]byte(data))
	L.Push(lua.LString(hex.EncodeToString(hash[:])))
	return 1
}

// cryptoSHA512 computes SHA-512 hash of input
func cryptoSHA512(L *lua.LState) int {
	data := L.ToString(1)
	hash := sha512.Sum512([]byte(data))
	L.Push(lua.LString(hex.EncodeToString(hash[:])))
	return 1
}

// cryptoHash computes hash with specified algorithm
func cryptoHash(L *lua.LState) int {
	algorithm := L.ToString(1)
	data := L.ToString(2)
	
	var result string
	switch strings.ToLower(algorithm) {
	case "sha256":
		hash := sha256.Sum256([]byte(data))
		result = hex.EncodeToString(hash[:])
	case "sha384":
		hash := sha512.Sum384([]byte(data))
		result = hex.EncodeToString(hash[:])
	case "sha512":
		hash := sha512.Sum512([]byte(data))
		result = hex.EncodeToString(hash[:])
	default:
		L.Push(lua.LNil)
		L.Push(lua.LString("unsupported algorithm: " + algorithm))
		return 2
	}
	
	L.Push(lua.LString(result))
	return 1
}

// cryptoDeepHash computes SHA-384 hash of nested data structures
func cryptoDeepHash(L *lua.LState) int {
	value := L.Get(1)
	algorithm := L.OptString(2, "sha384")
	
	// Serialize the value to a canonical string representation
	serialized, err := serializeForHashing(value)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("failed to serialize: " + err.Error()))
		return 2
	}
	
	var result string
	switch strings.ToLower(algorithm) {
	case "sha256":
		hash := sha256.Sum256([]byte(serialized))
		result = hex.EncodeToString(hash[:])
	case "sha384":
		hash := sha512.Sum384([]byte(serialized))
		result = hex.EncodeToString(hash[:])
	case "sha512":
		hash := sha512.Sum512([]byte(serialized))
		result = hex.EncodeToString(hash[:])
	default:
		L.Push(lua.LNil)
		L.Push(lua.LString("unsupported algorithm: " + algorithm))
		return 2
	}
	
	L.Push(lua.LString(result))
	return 1
}

// serializeForHashing converts Lua value to canonical string for consistent hashing
func serializeForHashing(lv lua.LValue) (string, error) {
	switch v := lv.(type) {
	case lua.LString:
		return string(v), nil
	case lua.LNumber:
		return fmt.Sprintf("%g", float64(v)), nil
	case lua.LBool:
		if bool(v) {
			return "true", nil
		}
		return "false", nil
	case *lua.LNilType:
		return "null", nil
	case *lua.LTable:
		// Check if it's an array or object
		isArray := true
		maxIndex := 0
		count := 0
		
		v.ForEach(func(k, val lua.LValue) {
			count++
			if num, ok := k.(lua.LNumber); ok {
				if int(num) > maxIndex {
					maxIndex = int(num)
				}
			} else {
				isArray = false
			}
		})
		
		if isArray && count > 0 && maxIndex == count {
			// Serialize as array
			var items []string
			for i := 1; i <= maxIndex; i++ {
				val := v.RawGetInt(i)
				serialized, err := serializeForHashing(val)
				if err != nil {
					return "", err
				}
				items = append(items, serialized)
			}
			return "[" + strings.Join(items, ",") + "]", nil
		} else {
			// Serialize as object with sorted keys
			var pairs []string
			keys := []string{}
			
			// Collect all keys
			v.ForEach(func(k, val lua.LValue) {
				if str, ok := k.(lua.LString); ok {
					keys = append(keys, string(str))
				} else if num, ok := k.(lua.LNumber); ok {
					keys = append(keys, fmt.Sprintf("%g", float64(num)))
				}
			})
			
			// Sort keys for consistent ordering
			sort.Strings(keys)
			
			// Build key-value pairs
			for _, key := range keys {
				val := v.RawGetString(key)
				if val == lua.LNil {
					// Try as number
					if num, err := strconv.ParseFloat(key, 64); err == nil {
						val = v.RawGetInt(int(num))
					}
				}
				
				serialized, err := serializeForHashing(val)
				if err != nil {
					return "", err
				}
				pairs = append(pairs, fmt.Sprintf("\"%s\":%s", key, serialized))
			}
			
			return "{" + strings.Join(pairs, ",") + "}", nil
		}
	default:
		return "", fmt.Errorf("unsupported type: %T", lv)
	}
}

// cryptoDigest computes hash digest with specified algorithm
func cryptoDigest(L *lua.LState) int {
	algorithm := L.ToString(1)
	data := L.ToString(2)
	
	if algorithm == "" || data == "" {
		L.Push(lua.LNil)
		L.Push(lua.LString("missing algorithm or data"))
		return 2
	}
	
	var result []byte
	switch strings.ToLower(algorithm) {
	case "sha256":
		hash := sha256.Sum256([]byte(data))
		result = hash[:]
	case "sha384":
		hash := sha512.Sum384([]byte(data))
		result = hash[:]
	case "sha512":
		hash := sha512.Sum512([]byte(data))
		result = hash[:]
	default:
		L.Push(lua.LNil)
		L.Push(lua.LString("unsupported algorithm: " + algorithm))
		return 2
	}
	
	// Return as hex string (similar to existing hash functions)
	L.Push(lua.LString(hex.EncodeToString(result)))
	return 1
}

// cryptoBase64URLDecode decodes a base64url encoded string
func cryptoBase64URLDecode(L *lua.LState) int {
	encoded := L.ToString(1)
	
	if encoded == "" {
		L.Push(lua.LNil)
		L.Push(lua.LString("missing encoded string"))
		return 2
	}
	
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("failed to decode: " + err.Error()))
		return 2
	}
	
	L.Push(lua.LString(string(decoded)))
	return 1
}