package httpserver

import (
	"bufio"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"strings"

	"go.n16f.net/ejson"
	"go.n16f.net/program"
	"golang.org/x/crypto/sha3"
)

type HashAlgorithm string

const (
	HashAlgorithmSHA256  = "SHA-256"
	HashAlgorithmSHA512  = "SHA-512"
	HashAlgorithmSHA3256 = "SHA3-256"
	HashAlgorithmSHA3512 = "SHA3-512"
)

var HashAlgorithmValues = []HashAlgorithm{
	HashAlgorithmSHA256,
	HashAlgorithmSHA512,
	HashAlgorithmSHA3256,
	HashAlgorithmSHA3512,
}

func (a HashAlgorithm) HashFunction() func() hash.Hash {
	var fn func() hash.Hash

	switch a {
	case HashAlgorithmSHA256:
		fn = sha256.New
	case HashAlgorithmSHA512:
		fn = sha512.New
	case HashAlgorithmSHA3256:
		fn = sha3.New256
	case HashAlgorithmSHA3512:
		fn = sha3.New512
	default:
		program.Panicf("unhandled hash algorithm %q", a)
	}

	return fn
}

type SecretsCfg struct {
	Hash HashAlgorithm   `json:"hash,omitempty"`
	HMAC *HMACSecretsCfg `json:"hmac,omitempty"`
}

func (cfg *SecretsCfg) ValidateJSON(v *ejson.Validator) {
	if cfg.Hash != "" && cfg.HMAC != nil {
		v.AddError(nil, "invalid_configuration",
			"cannot provide both a hash algorithm and HMAC configuration")
	}

	if cfg.Hash != "" {
		v.CheckStringValue("hash", cfg.Hash, HashAlgorithmValues)
	}

	v.CheckOptionalObject("hmac", cfg.HMAC)
}

type HMACSecretsCfg struct {
	Hash HashAlgorithm `json:"hash"`
	Key  []byte        `json:"key"`
}

func (cfg *HMACSecretsCfg) ValidateJSON(v *ejson.Validator) {
	v.CheckStringValue("hash", cfg.Hash, HashAlgorithmValues)
	v.CheckArrayNotEmpty("key", cfg.Key)
}

type AuthCfg struct {
	Secrets *SecretsCfg `json:"secrets,omitempty"`
	Realm   string      `json:"realm,omitempty"`

	Basic  *BasicAuthCfg  `json:"basic,omitempty"`
	Bearer *BearerAuthCfg `json:"bearer,omitempty"`
}

func (cfg *AuthCfg) ValidateJSON(v *ejson.Validator) {
	v.CheckOptionalObject("secrets", cfg.Secrets)

	if cfg.Basic != nil && cfg.Bearer != nil {
		v.AddError(nil, "invalid_configuration",
			"cannot provide multiple authentication method")
	}

	v.CheckOptionalObject("basic", cfg.Basic)
	v.CheckOptionalObject("bearer", cfg.Bearer)
}

type BasicAuthCfg struct {
	Credentials        []string `json:"credentials,omitempty"`
	CredentialFilePath string   `json:"credential_file_path,omitempty"`
}

func (cfg *BasicAuthCfg) ValidateJSON(v *ejson.Validator) {
	if cfg.CredentialFilePath != "" && len(cfg.Credentials) > 0 {
		v.AddError(nil, "invalid_configuration",
			"cannot provide both a credential file path and a list "+
				"of credentials")
	}
}

type BearerAuthCfg struct {
	Tokens        []string `json:"tokens,omitempty"`
	TokenFilePath string   `json:"token_file_path,omitempty"`
}

func (cfg *BearerAuthCfg) ValidateJSON(v *ejson.Validator) {
	if cfg.TokenFilePath != "" && len(cfg.Tokens) > 0 {
		v.AddError(nil, "invalid_configuration",
			"cannot provide both a token file path and a list of tokens")
	}
}

type Auth interface {
	Init(*AuthCfg) error
	AuthenticateRequest(*RequestContext) error
}

func NewAuth(cfg *AuthCfg) (Auth, error) {
	if cfg.Secrets == nil {
		cfg.Secrets = &SecretsCfg{
			Hash: HashAlgorithmSHA256,
		}
	}

	var auth Auth

	switch {
	case cfg.Basic != nil:
		auth = &BasicAuth{}
	case cfg.Bearer != nil:
		auth = &BearerAuth{}
	default:
		program.Panicf("incomplete authentication configuration")
	}

	if err := auth.Init(cfg); err != nil {
		return nil, err
	}

	return auth, nil
}

func transformAuthSecret(secret string, authCfg *AuthCfg) string {
	cfg := authCfg.Secrets

	var secret2 string

	switch {
	case cfg.Hash != "":
		secret2 = transformAuthSecretHash(secret, cfg.Hash)
	case cfg.HMAC != nil:
		secret2 = transformAuthSecretHMAC(secret, cfg.HMAC)
	default:
		program.Panicf("incomplete secrets configuration")
	}

	return secret2
}

func transformAuthSecretHash(secret string, hashAlgorithm HashAlgorithm) string {
	fn := hashAlgorithm.HashFunction()
	h := fn()
	h.Write([]byte(secret))
	sum := h.Sum(nil)
	return hex.EncodeToString(sum)
}

func transformAuthSecretHMAC(secret string, hmacCfg *HMACSecretsCfg) string {
	fn := hmacCfg.Hash.HashFunction()
	mac := hmac.New(fn, hmacCfg.Key)
	mac.Write([]byte(secret))
	sum := mac.Sum(nil)
	return hex.EncodeToString(sum)
}

func loadAuthSecretFile(filePath string) (map[string]struct{}, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot open %q: %w", filePath, err)
	}
	defer file.Close()

	r := bufio.NewReader(file)
	row := 1
	secrets := make(map[string]struct{})

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, fmt.Errorf("cannot read %q: %w", filePath, err)
		}

		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		secrets[line] = struct{}{}
		row++
	}

	return secrets, nil
}
