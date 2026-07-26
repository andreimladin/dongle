// Package auth is the host's credential store: it obtains and stores credentials
// so plugins never run their own login flow.
//
// The store here is a JSON file and Login mints a fake token — both are stubs
// for the PoC. In production, replace readStore/save with the OS keychain (macOS
// Keychain, Windows Credential Manager, libsecret), and replace Login's body
// with the real Entra ID (Azure AD) device-code flow.
//
// NOTE: this is the "5a" stopping point — store + login/logout only. Credential
// injection into the plugin process (Prepare) is the next step and is not wired
// in yet.
package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/andreimladin/dongle/internal/state"
)

type store struct {
	Tokens map[string]string `json:"tokens"` // provider -> token
}

func credPath() string { return filepath.Join(state.DataDir(), "credentials.json") }

func readStore() (*store, error) {
	b, err := os.ReadFile(credPath())
	if os.IsNotExist(err) {
		return &store{Tokens: map[string]string{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var s store
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	if s.Tokens == nil {
		s.Tokens = map[string]string{}
	}
	return &s, nil
}

func (s *store) save() error {
	if err := os.MkdirAll(state.DataDir(), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(credPath(), b, 0o600) // secret -> owner-only
}

// Login obtains and persists a credential for a provider.
//
// STUB: mints a fake token. Replace this body with the Entra ID device-code
// flow: request a device code, print the verification URL + code, poll the
// token endpoint while the user completes password + Authenticator number-match,
// then store the returned access token.
func Login(provider string) error {
	s, err := readStore()
	if err != nil {
		return err
	}
	s.Tokens[provider] = fmt.Sprintf("demo-token-%s-%d", provider, time.Now().Unix())
	return s.save()
}

func Logout(provider string) error {
	s, err := readStore()
	if err != nil {
		return err
	}
	delete(s.Tokens, provider)
	return s.save()
}

// token looks up a stored credential. Unused until the next step (credential
// injection) wires it into dispatch; it's the seam that step builds on.
func token(provider string) (string, bool) {
	s, err := readStore()
	if err != nil {
		return "", false
	}
	t, ok := s.Tokens[provider]
	return t, ok
}
