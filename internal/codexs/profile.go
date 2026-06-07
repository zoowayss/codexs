package codexs

import (
	"errors"
	"fmt"
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
		return errors.New("profile name is required")
	}
	if !profileNamePattern.MatchString(name) {
		return errors.New("profile name must start with a letter or digit and contain only letters, digits, dot, underscore, or hyphen")
	}
	return nil
}

func normalizeBaseURL(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", errors.New("base URL is required")
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("invalid base URL: %w", err)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", errors.New("base URL must use http or https")
	}
	if parsed.Host == "" {
		return "", errors.New("base URL must include a host")
	}
	return value, nil
}
