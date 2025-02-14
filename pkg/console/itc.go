package console

import (
	"fmt"
	"strconv"

	"github.com/dop251/goja"
	"github.com/intelchain-itc/intelchain/accounts"
	"github.com/intelchain-itc/intelchain/accounts/keystore"
	"github.com/intelchain-itc/intelchain/crypto/hash"
)

func signMessageWithPassword(keyStore *keystore.KeyStore, account accounts.Account, password string, data []byte) (sign []byte, err error) {
	signData := append([]byte("\x19Ethereum Signed Message:\n" + strconv.Itoa(len(data))))
	msgHash := hash.Keccak256(append(signData, data...))

	sign, err = keyStore.SignHashWithPassphrase(account, password, msgHash)
	if err != nil {
		return nil, err
	}

	if len(sign) != 65 {
		return nil, fmt.Errorf("sign error")
	}

	sign[64] += 0x1b
	return sign, nil
}

func getStringFromJsObjWithDefault(o *goja.Object, key string, def string) string {
	get := o.Get(key)
	if get == nil {
		return def
	} else {
		return get.String()
	}
}
