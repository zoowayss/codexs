package codexs

import "fmt"

// ErrProfileNotFound is returned when a profile does not exist
type ErrProfileNotFound struct {
	Name string
}

func (e ErrProfileNotFound) Error() string {
	return fmt.Sprintf("profile %q does not exist", e.Name)
}

// ErrProfileExists is returned when attempting to create a profile that already exists
type ErrProfileExists struct {
	Name string
}

func (e ErrProfileExists) Error() string {
	return fmt.Sprintf("profile %q already exists; use update to change it", e.Name)
}

// ErrSessionNotFound is returned when a session is not found in any profile
type ErrSessionNotFound struct {
	SessionID string
}

func (e ErrSessionNotFound) Error() string {
	return fmt.Sprintf("session %q not found in any profile", e.SessionID)
}

// ErrInvalidProfileName is returned when a profile name is invalid
type ErrInvalidProfileName struct {
	Name   string
	Reason string
}

func (e ErrInvalidProfileName) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("invalid profile name %q: %s", e.Name, e.Reason)
	}
	return fmt.Sprintf("invalid profile name %q", e.Name)
}

// ErrInvalidBaseURL is returned when a base URL is invalid
type ErrInvalidBaseURL struct {
	URL    string
	Reason string
}

func (e ErrInvalidBaseURL) Error() string {
	return fmt.Sprintf("invalid base URL %q: %s", e.URL, e.Reason)
}
