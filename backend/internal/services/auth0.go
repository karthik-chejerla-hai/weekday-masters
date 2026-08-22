package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// Auth0Profile is the verified user profile returned by Auth0's /userinfo endpoint.
// These values come from Auth0, never from the client request body.
type Auth0Profile struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

// Auth0Service fetches verified profile data from an Auth0 tenant.
type Auth0Service struct {
	domain string
	client *http.Client
}

func NewAuth0Service(domain string) *Auth0Service {
	return &Auth0Service{
		domain: domain,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

var ErrProfileUnavailable = errors.New("could not retrieve verified profile from Auth0")

// FetchProfile calls Auth0's /userinfo with the caller's access token. The token must
// carry the openid/profile/email scopes, which the frontend requests at login.
func (s *Auth0Service) FetchProfile(accessToken string) (*Auth0Profile, error) {
	if s.domain == "" {
		return nil, errors.New("AUTH0_DOMAIN is not configured")
	}

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://%s/userinfo", s.domain), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrProfileUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: /userinfo returned %d", ErrProfileUnavailable, resp.StatusCode)
	}

	var profile Auth0Profile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrProfileUnavailable, err)
	}

	if profile.Sub == "" || profile.Email == "" {
		return nil, fmt.Errorf("%w: response missing sub or email", ErrProfileUnavailable)
	}

	return &profile, nil
}
