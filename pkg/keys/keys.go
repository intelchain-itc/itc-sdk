package keys

import (
	"fmt"
	"os"
	"path"
	"strings"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/intelchain-itc/go-sdk/pkg/address"
	"github.com/intelchain-itc/intelchain/accounts/keystore"

	// "github.com/ethereum/go-ethereum/crypto"

	homedir "github.com/mitchellh/go-homedir"
)

func checkAndMakeKeyDirIfNeeded() string {
	userDir, _ := homedir.Dir()
	itcCLIDir := path.Join(userDir, ".itc_cli", "keystore")
	if _, err := os.Stat(itcCLIDir); os.IsNotExist(err) {
		// Double check with Leo what is right file persmission
		os.Mkdir(itcCLIDir, 0700)
	}

	return itcCLIDir
}

func ListKeys(keystoreDir string) {
	itcCLIDir := checkAndMakeKeyDirIfNeeded()
	scryptN := keystore.StandardScryptN
	scryptP := keystore.StandardScryptP
	ks := keystore.NewKeyStore(itcCLIDir, scryptN, scryptP)
	// keystore.KeyStore
	allAccounts := ks.Accounts()
	fmt.Printf("Intelchain Address:%s File URL:\n", strings.Repeat(" ", ethCommon.AddressLength*2))
	for _, account := range allAccounts {
		fmt.Printf("%s\t\t %s\n", address.ToBech32(account.Address), account.URL)
	}
}

func AddNewKey(password string) {
	itcCLIDir := checkAndMakeKeyDirIfNeeded()
	scryptN := keystore.StandardScryptN
	scryptP := keystore.StandardScryptP
	ks := keystore.NewKeyStore(itcCLIDir, scryptN, scryptP)
	account, err := ks.NewAccount(password)
	if err != nil {
		fmt.Printf("new account error: %v\n", err)
	}
	fmt.Printf("account: %s\n", address.ToBech32(account.Address))
	fmt.Printf("URL: %s\n", account.URL)
}
