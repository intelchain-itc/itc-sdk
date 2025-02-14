package account

import (
	"fmt"
	"os"
	"path"

	"github.com/intelchain-itc/itc-sdk/pkg/common"
	"github.com/intelchain-itc/itc-sdk/pkg/store"
	"github.com/mitchellh/go-homedir"
)

// RemoveAccount - removes an account from the keystore
func RemoveAccount(name string) error {
	accountExists := store.DoesNamedAccountExist(name)

	if !accountExists {
		return fmt.Errorf("account %s doesn't exist", name)
	}

	uDir, _ := homedir.Dir()
	itcCLIDir := path.Join(uDir, common.DefaultConfigDirName, common.DefaultConfigAccountAliasesDirName)
	accountDir := fmt.Sprintf("%s/%s", itcCLIDir, name)
	os.RemoveAll(accountDir)

	return nil
}
