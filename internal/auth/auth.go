// Package auth is the host's credential broker: it obtains, stores, and injects
// credentials so plugins never run their own login flow.
//
// The store here is a JSON file and Login mints a fake token — both are stubs
// for the PoC. In production:
//   - replace readStore/save with the OS keychain (macOS Keychain, Windows
//     Credential Manager, libsecret), and
//   - replace Login's body with the real Entra ID (Azure AD) device-code flow,
//     storing {access, refresh, expiresAt} and refreshing silently before exec.
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

// Credential is what we persist per provider. Today only Token is meaningful;
// the expiry fields are here so the Entra refresh logic drops in without a
// schema change.
type Credential struct {
	Token     string    `json:"token"`
	Refresh   string    `json:"refresh,omitempty"`
	ExpiresAt time.Time `json:"expiresAt,omitempty"`
}

type store struct {
	Tokens map[string]Credential `json:"tokens"` // provider -> credential
}

func credPath() string { return filepath.Join(state.DataDir(), "credentials.json") }

func readStore() (*store, error) {
	b, err := os.ReadFile(credPath())
	if os.IsNotExist(err) {
		return &store{Tokens: map[string]Credential{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var s store
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	if s.Tokens == nil {
		s.Tokens = map[string]Credential{}
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
// then store the returned access + refresh tokens and expiry.
func Login(provider string) error {
	s, err := readStore()
	if err != nil {
		return err
	}
	s.Tokens[provider] = Credential{
		Token:     fmt.Sprintf("demo-token-%s-%d", provider, time.Now().Unix()),
		ExpiresAt: time.Now().Add(time.Hour),
	}
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

// token returns a valid credential for a provider.
//
// TODO(entra): when ExpiresAt is in the past and Refresh is set, use the refresh
// token to obtain a new access token here, silently (no user interaction).
func token(provider string) (string, bool) {
	s, err := readStore()
	if err != nil {
		return "", false
	}
	c, ok := s.Tokens[provider]
	if !ok || c.Token == "" {
		return "", false
	}
	return c.Token, true
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
// Missing credentials return an error naming the provider to log in to. Secrets
// marked injectAs:"file" go to a per-invocation 0600 temp file (kept out of the
// process env); injectAs:"env" sets the declared env var.
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
		in.cleanup = append(in.cleanup, path, dir) // remove file, then the temp dir
		in.Env = append(in.Env, "DONGLE_CREDENTIALS_FILE="+path)
	}
	return in, nil
}
