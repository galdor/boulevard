package http

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
	"slices"
	"strings"

	"go.n16f.net/bcl"
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

var HashAlgorithmValues = []string{
	string(HashAlgorithmSHA256),
	string(HashAlgorithmSHA512),
	string(HashAlgorithmSHA3256),
	string(HashAlgorithmSHA3512),
}

func (a *HashAlgorithm) ReadBCLValue(v *bcl.Value) error {
	var s string

	switch v.Type() {
	case bcl.ValueTypeString:
		s = v.Content.(bcl.String).String
	default:
		return bcl.NewValueTypeError(v, bcl.ValueTypeString)
	}

	if !slices.Contains(HashAlgorithmValues, s) {
		return fmt.Errorf("invalid hash algorithm")
	}

	*a = HashAlgorithm(s)

	return nil
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
		program.Panic("unhandled hash algorithm %q", a)
	}

	return fn
}

type SecretsCfg struct {
	Hash HashAlgorithm
	HMAC *HMACSecretsCfg
}

func (cfg *SecretsCfg) ReadBCLElement(block *bcl.Element) error {
	block.CheckElementsOneOf("hash", "hmac")

	block.MaybeEntryValues("hash", &cfg.Hash)

	if entry := block.FindEntry("hmac"); entry != nil {
		cfg.HMAC = new(HMACSecretsCfg)
		entry.Values("hmac", &cfg.HMAC.Hash, &cfg.HMAC.Key)
	}

	return nil
}

type HMACSecretsCfg struct {
	Hash HashAlgorithm
	Key  []byte
}

type AuthCfg struct {
	Secrets *SecretsCfg
	Realm   string

	Basic  *BasicAuthCfg
	Bearer *BearerAuthCfg
}

func (cfg *AuthCfg) ReadBCLElement(block *bcl.Element) error {
	block.MaybeBlock("secrets", &cfg.Secrets)
	block.MaybeEntryValues("realm", &cfg.Realm)

	block.CheckBlocksOneOf("basic", "bearer")
	block.MaybeBlock("basic", &cfg.Basic)
	block.MaybeBlock("bearer", &cfg.Bearer)

	return nil
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
		program.Panic("incomplete authentication configuration")
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
		program.Panic("incomplete secrets configuration")
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
