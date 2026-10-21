package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTService struct {
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
	stateStore *StateStore
	mu         sync.RWMutex
}

type StateStore struct {
	mu     sync.RWMutex
	states map[string]StateData
}

type StateData struct {
	Created      time.Time
	ExpiresAt    time.Time
	Binding      string
	CodeVerifier string
	Nonce        string
}

const maxPendingStates = 10000

type SessionClaims struct {
	Username        string   `json:"username"`
	Email           string   `json:"email"`
	Name            string   `json:"name"`
	Roles           []string `json:"roles"`
	Teams           []string `json:"teams,omitempty"`
	AuthMethod      string   `json:"auth_method,omitempty"`
	SecurityVersion int      `json:"security_version,omitempty"`
	jwt.RegisteredClaims
}

func NewJWTService() (*JWTService, error) {
	return NewJWTServiceWithKey("")
}

func NewJWTServiceWithKey(privateKeyPEM string) (*JWTService, error) {
	var privateKey ed25519.PrivateKey
	var publicKey ed25519.PublicKey
	var err error

	if privateKeyPEM != "" {
		// Parse the provided PEM-encoded private key
		privateKey, err = parseEd25519PrivateKeyFromPEM(privateKeyPEM)
		if err != nil {
			return nil, fmt.Errorf("failed to parse Ed25519 private key: %w", err)
		}
		publicKey = privateKey.Public().(ed25519.PublicKey)
	} else {
		// Generate a new Ed25519 key pair if no key is provided
		publicKey, privateKey, err = ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("failed to generate Ed25519 key: %w", err)
		}
	}

	return &JWTService{
		privateKey: privateKey,
		publicKey:  publicKey,
		stateStore: &StateStore{
			states: make(map[string]StateData),
		},
	}, nil
}

func parseEd25519PrivateKeyFromPEM(privateKeyPEM string) (ed25519.PrivateKey, error) {
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	// Try to parse as PKCS#8 format (standard format from openssl genpkey -algorithm ED25519)
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err == nil {
		// Successfully parsed as PKCS#8, check if it's an Ed25519 key
		if ed25519Key, ok := key.(ed25519.PrivateKey); ok {
			return ed25519Key, nil
		}
		return nil, fmt.Errorf("PKCS#8 key is not an Ed25519 key")
	}

	// Fallback: Check if it's raw Ed25519 private key bytes (64 bytes)
	if len(block.Bytes) == ed25519.PrivateKeySize {
		return block.Bytes, nil
	}

	return nil, fmt.Errorf("invalid Ed25519 private key format: not PKCS#8 and not raw %d bytes (got %d bytes)",
		ed25519.PrivateKeySize, len(block.Bytes))
}

func (j *JWTService) GenerateStateTokenForBinding(binding string) (string, error) {
	return j.generateState(binding, "", "")
}

func (j *JWTService) GenerateOIDCState(binding, verifier, nonce string) (string, error) {
	return j.generateState(binding, verifier, nonce)
}

func (j *JWTService) generateState(binding, verifier, nonce string) (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("failed to generate state token: %w", err)
	}

	token := base64.URLEncoding.EncodeToString(tokenBytes)

	j.stateStore.mu.Lock()
	defer j.stateStore.mu.Unlock()

	now := time.Now()
	for existing, state := range j.stateStore.states {
		if now.After(state.ExpiresAt) {
			delete(j.stateStore.states, existing)
		}
	}
	if len(j.stateStore.states) >= maxPendingStates {
		return "", fmt.Errorf("too many pending OIDC states")
	}
	j.stateStore.states[token] = StateData{
		Created:      now,
		ExpiresAt:    now.Add(10 * time.Minute),
		Binding:      stateBinding(binding),
		CodeVerifier: verifier,
		Nonce:        nonce,
	}

	go j.cleanupExpiredStates()

	return token, nil
}

func (j *JWTService) ValidateAndConsumeStateForBinding(token, binding string) bool {
	_, ok := j.ConsumeStateForBinding(token, binding)
	return ok
}

func (j *JWTService) ConsumeStateForBinding(token, binding string) (StateData, bool) {
	j.stateStore.mu.Lock()
	defer j.stateStore.mu.Unlock()

	state, exists := j.stateStore.states[token]
	if !exists {
		return StateData{}, false
	}

	if time.Now().After(state.ExpiresAt) {
		delete(j.stateStore.states, token)
		return StateData{}, false
	}
	// Empty bindings are retained for callers of the legacy API; the OIDC
	// route always creates a bound state and also sets the browser cookie.
	if state.Binding != "" && state.Binding != stateBinding(binding) {
		return StateData{}, false
	}

	delete(j.stateStore.states, token)
	return state, true
}

func stateBinding(value string) string {
	if value == "" {
		return ""
	}
	h := sha256.Sum256([]byte(value))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

func (j *JWTService) cleanupExpiredStates() {
	j.stateStore.mu.Lock()
	defer j.stateStore.mu.Unlock()

	now := time.Now()
	for token, state := range j.stateStore.states {
		if now.After(state.ExpiresAt) {
			delete(j.stateStore.states, token)
		}
	}
}

func (j *JWTService) GenerateToken(userInfo *UserInfo, sessionMaxAge int) (string, error) {
	j.mu.RLock()
	defer j.mu.RUnlock()

	if j.privateKey == nil {
		return "", fmt.Errorf("private key not initialized")
	}

	now := time.Now()
	expiresAt := now.Add(time.Duration(sessionMaxAge) * time.Second)

	claims := SessionClaims{
		Username:        userInfo.Username,
		Email:           userInfo.Email,
		Name:            userInfo.Name,
		Roles:           userInfo.Roles,
		Teams:           userInfo.Teams,
		AuthMethod:      userInfo.AuthMethod,
		SecurityVersion: userInfo.SecurityVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	tokenString, err := token.SignedString(j.privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

func (j *JWTService) ValidateToken(tokenString string) (*SessionClaims, error) {
	j.mu.RLock()
	defer j.mu.RUnlock()

	if j.publicKey == nil {
		return nil, fmt.Errorf("public key not initialized")
	}

	token, err := jwt.ParseWithClaims(tokenString, &SessionClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodEd25519); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return j.publicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(*SessionClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}


