package evalrun

import (
	"errors"
	"testing"

	"github.com/zalando/go-keyring"
)

// TestFromKeyring checks the lookup cech eval makes: the service cech
// and the variable's name as the user name, an entry that is not there
// as no credential, and any other failure as an error (criterion 20).
func TestFromKeyring(t *testing.T) {
	defer func(g func(string, string) (string, error)) { keyringGet = g }(keyringGet)
	keyringGet = func(service, user string) (string, error) {
		switch {
		case service != KeyringService:
			return "", errors.New("wrong service " + service)
		case user == "CLAUDE_CODE_OAUTH_TOKEN":
			return "token", nil
		case user == "CODEX_API_KEY":
			return "", errors.New("no keyring")
		}
		return "", keyring.ErrNotFound
	}
	if v, err := FromKeyring("CLAUDE_CODE_OAUTH_TOKEN"); v != "token" || err != nil {
		t.Errorf("a stored entry: %q, %v", v, err)
	}
	if v, err := FromKeyring("ANTHROPIC_API_KEY"); v != "" || err != nil {
		t.Errorf("an entry that is not there: %q, %v; want none and no error", v, err)
	}
	if _, err := FromKeyring("CODEX_API_KEY"); err == nil {
		t.Error("a keyring that fails gave no error")
	}
}
