package codexs

import (
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	appID          = "codexs"
	stateVersion   = 1
	defaultBaseURL = "https://api.openai.com/v1"
)

var profileNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

type Profile struct {
	Name      string    `json:"name"`
	BaseURL   string    `json:"base_url"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type State struct {
	Version  int                `json:"version"`
	Profiles map[string]Profile `json:"profiles"`
}

func newState() State {
	return State{
		Version:  stateVersion,
		Profiles: make(map[string]Profile),
	}
}

func validateProfileName(name string) error {
	if name == "" {
		return ErrInvalidProfileName{Name: name, Reason: "profile name is required"}
	}
	if !profileNamePattern.MatchString(name) {
		return ErrInvalidProfileName{
			Name:   name,
			Reason: "must start with a letter or digit and contain only letters, digits, dot, underscore, or hyphen",
		}
	}
	return nil
}

func normalizeBaseURL(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", ErrInvalidBaseURL{URL: value, Reason: "base URL is required"}
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", ErrInvalidBaseURL{URL: value, Reason: err.Error()}
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", ErrInvalidBaseURL{URL: value, Reason: "must use http or https"}
	}
	if parsed.Host == "" {
		return "", ErrInvalidBaseURL{URL: value, Reason: "must include a host"}
	}
	return value, nil
}
