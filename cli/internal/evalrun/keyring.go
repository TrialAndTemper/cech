package evalrun

import (
	"errors"

	"github.com/zalando/go-keyring"
)

// KeyringService is the service under which cech eval looks up its
// credentials in the system keyring: GNOME Keyring or KWallet through
// the Secret Service, the macOS keychain, or the Windows Credential
// Manager. Each credential is an entry whose user name is the
// environment variable it stands for, such as CLAUDE_CODE_OAUTH_TOKEN.
const KeyringService = "cech"

// keyringGet is the keyring lookup; tests replace it.
var keyringGet = keyring.Get

// FromKeyring returns the secret stored for name, or "" when the keyring
// has none, so cech eval tries the next credential (criterion 20). An
// error says the keyring could not be read, which the message for a
// harness without a credential gives.
func FromKeyring(name string) (string, error) {
	v, err := keyringGet(KeyringService, name)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", nil
		}
		return "", err
	}
	return v, nil
}
