package middleware

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/testsupport"
)

// These tests exercise the real signature/audience/issuer checks rather than
// stubbing them out — the token path is the only thing standing between an
// anonymous request and an approved member's data.
//
// getJWKS reaches Auth0 over HTTPS at a URL it builds itself, so there is no
// seam to point at an httptest server. Instead the tests seed the package-level
// JWKS cache, which is the same value a successful fetch would have produced.

const (
	testDomain   = "rally-test.au.auth0.com"
	testAudience = "https://api.rally.test"
	testKeyID    = "test-key-1"
)

type signingKey struct {
	private *rsa.PrivateKey
	jwks    *JWKS
}

// newSigningKey builds an RSA key plus the JWKS entry Auth0 would publish for
// it. getKeyFromJWKS reads x5c[0] and wraps it in a CERTIFICATE PEM, so the
// JWKS has to carry a real DER certificate, not a bare public key.
func newSigningKey(t *testing.T, kid string) *signingKey {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: testDomain},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	return &signingKey{
		private: key,
		jwks: &JWKS{Keys: []JSONWebKey{{
			Kty: "RSA",
			Kid: kid,
			Use: "sig",
			X5c: []string{base64.StdEncoding.EncodeToString(der)},
		}}},
	}
}

// install seeds the JWKS cache and restores it when the test ends, so the cache
// cannot leak between tests.
func (k *signingKey) install(t *testing.T) {
	t.Helper()

	prevCache, prevTime := jwksCache, jwksCacheTime
	t.Cleanup(func() { jwksCache, jwksCacheTime = prevCache, prevTime })

	jwksCache, jwksCacheTime = k.jwks, time.Now()
}

// sign issues an RS256 token, applying any mutations to the default-valid claims.
func (k *signingKey) sign(t *testing.T, kid string, mutate func(jwt.MapClaims)) string {
	t.Helper()

	claims := jwt.MapClaims{
		"sub": "auth0|test-subject",
		"aud": testAudience,
		"iss": "https://" + testDomain + "/",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Add(-time.Minute).Unix(),
	}
	if mutate != nil {
		mutate(claims)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid

	signed, err := token.SignedString(k.private)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return signed
}

func testConfig() Auth0Config {
	return Auth0Config{Domain: testDomain, Audience: testAudience}
}

func TestValidateToken_AcceptsAProperlySignedToken(t *testing.T) {
	key := newSigningKey(t, testKeyID)
	key.install(t)

	claims, err := ValidateToken(testConfig(), key.sign(t, testKeyID, nil))
	if err != nil {
		t.Fatalf("expected a valid token to be accepted, got %v", err)
	}
	if claims["sub"] != "auth0|test-subject" {
		t.Fatalf("expected subject to survive validation, got %v", claims["sub"])
	}
}

func TestValidateToken_Rejections(t *testing.T) {
	key := newSigningKey(t, testKeyID)
	otherKey := newSigningKey(t, testKeyID)

	cases := []struct {
		name  string
		token func(t *testing.T) string
	}{
		{
			name:  "not a JWT at all",
			token: func(*testing.T) string { return "not-a-token" },
		},
		{
			name: "signed by a key that is not in the JWKS",
			// Same kid, different private key: the signature must not verify.
			token: func(t *testing.T) string { return otherKey.sign(t, testKeyID, nil) },
		},
		{
			name:  "kid is not published in the JWKS",
			token: func(t *testing.T) string { return key.sign(t, "unknown-kid", nil) },
		},
		{
			name: "audience belongs to a different API",
			token: func(t *testing.T) string {
				return key.sign(t, testKeyID, func(c jwt.MapClaims) { c["aud"] = "https://someone-elses.api" })
			},
		},
		{
			name: "issuer is not our Auth0 tenant",
			token: func(t *testing.T) string {
				return key.sign(t, testKeyID, func(c jwt.MapClaims) { c["iss"] = "https://evil.example.com/" })
			},
		},
		{
			name: "token has expired",
			token: func(t *testing.T) string {
				return key.sign(t, testKeyID, func(c jwt.MapClaims) {
					c["exp"] = time.Now().Add(-time.Minute).Unix()
				})
			},
		},
		{
			name: "header carries no kid",
			token: func(t *testing.T) string {
				token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{"sub": "auth0|x"})
				signed, err := token.SignedString(key.private)
				if err != nil {
					t.Fatalf("failed to sign: %v", err)
				}
				return signed
			},
		},
		{
			name: "algorithm downgraded to HMAC",
			// The classic confusion attack: an attacker who knows the public key
			// resigns with HS256 and hopes the verifier accepts it.
			token: func(t *testing.T) string {
				token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
					"sub": "auth0|attacker",
					"aud": testAudience,
					"iss": "https://" + testDomain + "/",
					"exp": time.Now().Add(time.Hour).Unix(),
				})
				token.Header["kid"] = testKeyID
				signed, err := token.SignedString([]byte("secret"))
				if err != nil {
					t.Fatalf("failed to sign: %v", err)
				}
				return signed
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key.install(t)

			if _, err := ValidateToken(testConfig(), tc.token(t)); err == nil {
				t.Fatal("expected the token to be rejected, but it was accepted")
			}
		})
	}
}

func TestGetJWKS_ReturnsCachedKeysWithoutFetching(t *testing.T) {
	key := newSigningKey(t, testKeyID)
	key.install(t)

	// A blank domain would fail the fetch, so a result here proves the cache hit.
	jwks, err := getJWKS("")
	if err != nil {
		t.Fatalf("expected the cached JWKS to be returned, got %v", err)
	}
	if len(jwks.Keys) != 1 || jwks.Keys[0].Kid != testKeyID {
		t.Fatalf("expected the cached key, got %+v", jwks.Keys)
	}
}

func TestGetJWKS_ErrorsWhenDomainIsUnconfigured(t *testing.T) {
	prevCache, prevTime := jwksCache, jwksCacheTime
	t.Cleanup(func() { jwksCache, jwksCacheTime = prevCache, prevTime })
	jwksCache, jwksCacheTime = nil, time.Time{}

	if _, err := getJWKS(""); err == nil {
		t.Fatal("expected an error when AUTH0_DOMAIN is not configured")
	}
}

func TestGetKeyFromJWKS(t *testing.T) {
	jwks := &JWKS{Keys: []JSONWebKey{
		{Kid: "no-cert"},
		{Kid: "with-cert", X5c: []string{"CERTDATA"}},
	}}

	pem, err := getKeyFromJWKS(jwks, "with-cert")
	if err != nil {
		t.Fatalf("expected to find the key, got %v", err)
	}
	if pem != "-----BEGIN CERTIFICATE-----\nCERTDATA\n-----END CERTIFICATE-----" {
		t.Fatalf("unexpected PEM: %q", pem)
	}

	// A key with no x5c entry is unusable, and so is a kid we do not publish.
	if _, err := getKeyFromJWKS(jwks, "no-cert"); err == nil {
		t.Fatal("expected an error for a key with no certificate")
	}
	if _, err := getKeyFromJWKS(jwks, "absent"); err == nil {
		t.Fatal("expected an error for an unknown kid")
	}
}

func TestExtractBearerToken(t *testing.T) {
	cases := []struct {
		name    string
		header  string
		want    string
		wantErr bool
	}{
		{name: "well formed", header: "Bearer abc.def.ghi", want: "abc.def.ghi"},
		{name: "missing header", header: "", wantErr: true},
		{name: "no Bearer prefix", header: "abc.def.ghi", wantErr: true},
		{name: "wrong scheme", header: "Basic abc", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)
			if tc.header != "" {
				c.Request.Header.Set("Authorization", tc.header)
			}

			got, err := extractBearerToken(c)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error for header %q", tc.header)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

// routerWith mounts a probe handler behind the middleware under test and
// reports what the middleware put on the context.
func routerWith(mw gin.HandlerFunc) (*gin.Engine, *struct {
	auth0ID string
	token   string
	userID  string
}) {
	seen := &struct {
		auth0ID string
		token   string
		userID  string
	}{}

	r := gin.New()
	r.Use(mw)
	r.GET("/probe", func(c *gin.Context) {
		seen.auth0ID = c.GetString("auth0ID")
		seen.token = c.GetString("accessToken")
		if user, ok := c.Get("user"); ok {
			seen.userID = user.(*models.User).ID.String()
		}
		c.Status(http.StatusOK)
	})
	return r, seen
}

func do(r *gin.Engine, authHeader string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/probe", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	r.ServeHTTP(w, req)
	return w
}

func TestRequireValidToken_AdmitsAValidTokenWithNoUserRow(t *testing.T) {
	key := newSigningKey(t, testKeyID)
	key.install(t)

	token := key.sign(t, testKeyID, nil)
	r, seen := routerWith(RequireValidToken(testConfig()))

	if w := do(r, "Bearer "+token); w.Code != http.StatusOK {
		t.Fatalf("expected 200 for a valid token, got %d: %s", w.Code, w.Body)
	}
	// Registration handlers read the subject from here, never from the body.
	if seen.auth0ID != "auth0|test-subject" {
		t.Fatalf("expected the verified subject on the context, got %q", seen.auth0ID)
	}
	if seen.token != token {
		t.Fatal("expected the raw token to be stored for the /userinfo call")
	}
}

func TestRequireValidToken_RejectsMissingAndBadTokens(t *testing.T) {
	key := newSigningKey(t, testKeyID)

	for _, tc := range []struct {
		name   string
		header string
	}{
		{name: "no Authorization header", header: ""},
		{name: "not a Bearer token", header: "Basic dXNlcjpwYXNz"},
		{name: "garbage token", header: "Bearer nonsense"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key.install(t)
			r, _ := routerWith(RequireValidToken(testConfig()))

			if w := do(r, tc.header); w.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d", w.Code)
			}
		})
	}
}

func TestRequireValidToken_RejectsATokenWithNoSubject(t *testing.T) {
	key := newSigningKey(t, testKeyID)
	key.install(t)

	token := key.sign(t, testKeyID, func(c jwt.MapClaims) { delete(c, "sub") })
	r, _ := routerWith(RequireValidToken(testConfig()))

	if w := do(r, "Bearer "+token); w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for a subject-less token, got %d", w.Code)
	}
}

func TestAuthMiddleware_LoadsTheMatchingUser(t *testing.T) {
	testsupport.RequireDB(t)

	key := newSigningKey(t, testKeyID)
	key.install(t)

	user := models.User{
		ID:               uuid.New(),
		Auth0ID:          "auth0|test-subject",
		Email:            "member@example.com",
		Name:             "Member",
		Role:             models.RolePlayer,
		MembershipStatus: models.MembershipApproved,
	}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	r, seen := routerWith(AuthMiddleware(testConfig()))

	if w := do(r, "Bearer "+key.sign(t, testKeyID, nil)); w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body)
	}
	if seen.userID != user.ID.String() {
		t.Fatalf("expected the middleware to load user %s, got %q", user.ID, seen.userID)
	}
}

func TestAuthMiddleware_RejectsAValidTokenWithNoUserRow(t *testing.T) {
	testsupport.RequireDB(t)

	key := newSigningKey(t, testKeyID)
	key.install(t)

	// A token that validates but whose subject has never registered: the
	// difference between RequireValidToken and AuthMiddleware.
	r, _ := routerWith(AuthMiddleware(testConfig()))

	w := do(r, "Bearer "+key.sign(t, testKeyID, nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for an unregistered subject, got %d", w.Code)
	}
}
