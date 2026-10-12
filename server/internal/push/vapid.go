// SPDX-License-Identifier: AGPL-3.0-or-later

package push

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// Keys is the VAPID key pair that identifies our server to push services.
type Keys struct {
	Public  string `json:"public"`
	Private string `json:"private"`
}

// LoadOrCreateKeys reads the VAPID keys from path, generating and saving them on first run.
// Changing the keys invalidates every existing subscription, so the file must be kept.
func LoadOrCreateKeys(path string) (Keys, error) {
	b, err := os.ReadFile(path)
	if err == nil {
		var k Keys
		if err := json.Unmarshal(b, &k); err != nil || k.Public == "" || k.Private == "" {
			return Keys{}, fmt.Errorf("push: bad key file %s", path)
		}
		return k, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return Keys{}, err
	}
	priv, pub, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		return Keys{}, err
	}
	k := Keys{Public: pub, Private: priv}
	b, _ = json.Marshal(k)
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return Keys{}, err
	}
	return k, nil
}
