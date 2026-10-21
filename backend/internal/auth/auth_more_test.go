package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Mimic890/kuraban/internal/config"

	"golang.org/x/oauth2"
)

// newOIDCServerWithTokenAndUserInfo extends the minimal discovery stub with
// token and userinfo endpoints so ExchangeCode and GetUserInfo can be driven
// end-to-end without a real IdP.
func newOIDCServerWithTokenAndUserInfo(
	t *testing.T,
	tokenResp map[string]any,
	tokenStatus int,
	userInfoResp map[string]any,
	userInfoStatus int,
) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		doc := map[string]any{
			"issuer":                                srv.URL,
			"authorization_endpoint":                srv.URL + "/auth",
			"token_endpoint":                        srv.URL + "/token",
			"jwks_uri":                              srv.URL + "/jwks",
			"userinfo_endpoint":                     srv.URL + "/userinfo",
			"id_token_signing_alg_values_supported": []string{"RS256"},
			"response_types_supported":              []string{"code"},
			"subject_types_supported":               []string{"public"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys":[]}`))
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(tokenStatus)
		if tokenResp != nil {
			_ = json.NewEncoder(w).Encode(tokenResp)
		}
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(userInfoStatus)
		if userInfoResp != nil {
			_ = json.NewEncoder(w).Encode(userInfoResp)
		}
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestExchangeCode_OIDCDisabledReturnsError(t *testing.T) {
	svc := &Service{authConfig: &config.AuthConfig{}, serverConfig: &config.ServerConfig{}}
	if _, err := svc.ExchangeCode(context.Background(), "some-code"); err == nil {
		t.Fatal("expected error when OIDC not initialized")
	}
}

func TestExchangeCode_TokenEndpointErrorPropagates(t *testing.T) {
	srv := newOIDCServerWithTokenAndUserInfo(t,
		map[string]any{"error": "invalid_grant"}, http.StatusBadRequest,
		nil, http.StatusOK,
	)
	svc, err := NewAuthService(
		&config.AuthConfig{OIDC: config.OIDCConfig{
			Enabled:   true,
			ClientID:  "c",
			IssuerURL: srv.URL,
			Scopes:    []string{"openid"},
		}},
		&config.ServerConfig{RootURL: "https://example.test"},
	)
	if err != nil {
		t.Fatalf("NewAuthService: %v", err)
	}

	_, err = svc.ExchangeCode(context.Background(), "bad-code")
	if err == nil {
		t.Fatal("expected error for 400 from token endpoint")
	}
	if !strings.Contains(err.Error(), "failed to exchange code") {
		t.Errorf("error = %v, want wrap 'failed to exchange code'", err)
	}
}

func TestVerifyIDToken_OIDCDisabledReturnsError(t *testing.T) {
	svc := &Service{authConfig: &config.AuthConfig{}, serverConfig: &config.ServerConfig{}}
	if _, err := svc.VerifyIDToken(context.Background(), "tok"); err == nil {
		t.Fatal("expected error when OIDC not initialized")
	}
}

func TestVerifyIDToken_GarbageTokenRejected(t *testing.T) {
	// Discovery works; JWKS is empty so no signature can verify — any token
	// is rejected. This exercises the verifier error path.
	srv := newOIDCServerWithTokenAndUserInfo(t, nil, http.StatusOK, nil, http.StatusOK)
	svc, err := NewAuthService(
		&config.AuthConfig{OIDC: config.OIDCConfig{
			Enabled:   true,
			ClientID:  "c",
			IssuerURL: srv.URL,
			Scopes:    []string{"openid"},
		}},
		&config.ServerConfig{RootURL: "https://example.test"},
	)
	if err != nil {
		t.Fatalf("NewAuthService: %v", err)
	}

	_, err = svc.VerifyIDToken(context.Background(), "not-a-real-jwt")
	if err == nil {
		t.Fatal("expected verifier error for garbage token")
	}
}

func TestGetUserInfo_OIDCDisabledReturnsError(t *testing.T) {
	svc := &Service{authConfig: &config.AuthConfig{}, serverConfig: &config.ServerConfig{}}
	_, err := svc.GetUserInfo(context.Background(), &oauth2.Token{AccessToken: "x"})
	if err == nil {
		t.Fatal("expected error when OIDC not initialized")
	}
}

func TestGetUserInfo_ProviderErrorPropagates(t *testing.T) {
	srv := newOIDCServerWithTokenAndUserInfo(t,
		nil, http.StatusOK,
		map[string]any{"error": "invalid_token"}, http.StatusUnauthorized,
	)
	svc, err := NewAuthService(
		&config.AuthConfig{OIDC: config.OIDCConfig{
			Enabled:   true,
			ClientID:  "c",
			IssuerURL: srv.URL,
			Scopes:    []string{"openid"},
		}},
		&config.ServerConfig{RootURL: "https://example.test"},
	)
	if err != nil {
		t.Fatalf("NewAuthService: %v", err)
	}

	_, err = svc.GetUserInfo(context.Background(), &oauth2.Token{AccessToken: "bad"})
	if err == nil {
		t.Fatal("expected error from userinfo endpoint")
	}
	if !strings.Contains(err.Error(), "failed to get user info") {
		t.Errorf("error = %v", err)
	}
}

func TestGetUserInfo_HappyPath_ExtractsClaims(t *testing.T) {
	srv := newOIDCServerWithTokenAndUserInfo(t,
		nil, http.StatusOK,
		map[string]any{
			"sub":                "user-123",
			"preferred_username": "alice",
			"email":              "alice@example.com",
			"name":               "Alice Example",
			"resource_access": map[string]any{
				"garage": map[string]any{
					"roles": []any{"admin", "user"},
				},
			},
		},
		http.StatusOK,
	)
	svc, err := NewAuthService(
		&config.AuthConfig{OIDC: config.OIDCConfig{
			Enabled:           true,
			ClientID:          "c",
			IssuerURL:         srv.URL,
			Scopes:            []string{"openid"},
			UsernameAttribute: "preferred_username",
			EmailAttribute:    "email",
			NameAttribute:     "name",
			RoleAttributePath: "resource_access.garage.roles",
		}},
		&config.ServerConfig{RootURL: "https://example.test"},
	)
	if err != nil {
		t.Fatalf("NewAuthService: %v", err)
	}

	info, err := svc.GetUserInfo(context.Background(), &oauth2.Token{AccessToken: "good"})
	if err != nil {
		t.Fatalf("GetUserInfo: %v", err)
	}
	if info.Username != "alice" || info.Email != "alice@example.com" || info.Name != "Alice Example" {
		t.Errorf("got %+v, want alice/alice@example.com/Alice Example", info)
	}
	if len(info.Roles) != 2 || info.Roles[0] != "admin" {
		t.Errorf("Roles = %v, want [admin user]", info.Roles)
	}
}

func TestExtractClaim(t *testing.T) {
	cases := []struct {
		name   string
		claims map[string]interface{}
		key    string
		want   string
	}{
		{"empty key returns empty", map[string]interface{}{"a": "b"}, "", ""},
		{"missing key returns empty", map[string]interface{}{"a": "b"}, "missing", ""},
		{"non-string value returns empty", map[string]interface{}{"n": 42}, "n", ""},
		{"string value returned", map[string]interface{}{"email": "x@y"}, "email", "x@y"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractClaim(tc.claims, tc.key); got != tc.want {
				t.Errorf("extractClaim(%v, %q) = %q, want %q", tc.claims, tc.key, got, tc.want)
			}
		})
	}
}
