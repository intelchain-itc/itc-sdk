package common

import (
	"github.com/intelchain-itc/intelchain/accounts/keystore"
)

func KeyStoreForPath(p string) *keystore.KeyStore {
	return keystore.NewKeyStore(p, ScryptN, ScryptP)
}
