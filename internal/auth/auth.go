// Package auth is the host's credential broker: it obtains, stores, and injects
// credentials so plugins never run their own login flow.
//
// The store here is a JSON file for demo purposes. In production, replace
// readStore/writeStore with the OS keychain (macOS Keychain, Windows Credential
// Manager, libsecret), and replace Login's body with the real auth flow
// (OAuth device flow, API-token prompt, etc.).
package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/andreimladin/dongle/internal/manifest"
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
	if err := os.MkdirAll(state.DataDir(), 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(s, "", "  ")
	return os.WriteFile(credPath(), b, 0o600)
}

// Login obtains and persists a credential for a provider. Stub: it mints a fake
// token. Swap the token line for the provider's real flow.
func Login(provider string) error {
	s, err := readStore()
	if err != nil {
		return err
	}
	s.Tokens[provider] = fmt.Sprintf("demo-token-%s-%d", provider, time.Now().Unix())
	return s.save()
}

// Logout removes a stored credential.
func Logout(provider string) error {
	s, err := readStore()
	if err != nil {
		return err
	}
	delete(s.Tokens, provider)
	return s.save()
}

func token(provider string) (string, bool) {
	s, err := readStore()
	if err != nil {
		return "", false
	}
	t, ok := s.Tokens[provider]
	return t, ok
}

// Injection is everything the host adds to a plugin's process for one run.
type Injection struct {
	Env     []string // KEY=VALUE pairs appended to the plugin's environment
	cleanup []string // temp paths to remove once the plugin exits
}

// Cleanup removes any short-lived credential files after the plugin exits.
func (in *Injection) Cleanup() {
	for _, p := range in.cleanup {
		_ = os.Remove(p)
	}
}

// Prepare resolves every credential a plugin declares and builds the injection.
// If a required credential is missing, it returns an error naming the provider
// to log in to. Secrets marked injectAs:"file" are written to a per-invocation
// 0600 temp file (kept out of the process env); injectAs:"env" sets the
// declared env var.
func Prepare(m *manifest.Manifest, invocationID string) (*Injection, error) {
	in := &Injection{}
	fileCreds := map[string]string{} // provider -> token, for injectAs: file

	for _, a := range m.Auth {
		tok, ok := token(a.Provider)
		if !ok {
			in.Cleanup()
			return nil, fmt.Errorf("not logged in to %q — run: dongle login %s", a.Provider, a.Provider)
		}
		switch a.InjectAs {
		case "env":
			env := a.EnvVar
			if env == "" {
				env = "DONGLE_TOKEN_" + a.Provider
			}
			in.Env = append(in.Env, env+"="+tok)
		case "file", "":
			fileCreds[a.Provider] = tok
		default:
			in.Cleanup()
			return nil, fmt.Errorf("unknown injectAs %q for provider %s", a.InjectAs, a.Provider)
		}
	}

	if len(fileCreds) > 0 {
		dir, err := os.MkdirTemp("", "dongle-creds-"+invocationID+"-")
		if err != nil {
			return nil, err
		}
		path := filepath.Join(dir, "credentials.json")
		b, _ := json.Marshal(map[string]any{"tokens": fileCreds})
		if err := os.WriteFile(path, b, 0o600); err != nil {
			return nil, err
		}
		// Remove the file first, then the (now-empty) temp dir.
		in.cleanup = append(in.cleanup, path, dir)
		in.Env = append(in.Env, "DONGLE_CREDENTIALS_FILE="+path)
	}
	return in, nil
}
