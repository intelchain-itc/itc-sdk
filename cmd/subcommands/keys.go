package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/tyler-smith/go-bip39"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/intelchain-itc/itc-sdk/pkg/account"
	c "github.com/intelchain-itc/itc-sdk/pkg/common"
	"github.com/intelchain-itc/itc-sdk/pkg/keys"
	"github.com/intelchain-itc/itc-sdk/pkg/ledger"
	"github.com/intelchain-itc/itc-sdk/pkg/mnemonic"
	"github.com/intelchain-itc/itc-sdk/pkg/store"
	"github.com/intelchain-itc/itc-sdk/pkg/validation"
)

const (
	seedPhraseWarning = "**Important** write this seed phrase in a safe place, " +
		"it is the only way to recover your account if you ever forget your password\n\n"
)

var (
	quietImport            bool
	recoverFromMnemonic    bool
	userProvidesPassphrase bool
	passphraseFilePath     string
	passphrase             string
	blsFilePath            string
	blsShardID             uint32
	blsCount               uint32
	ppPrompt               = fmt.Sprintf(
		"prompt for passphrase, otherwise use default passphrase: \"`%s`\"", c.DefaultPassphrase,
	)
)

// getPassphrase fetches the correct passphrase depending on if a file is available to
// read from or if the user wants to enter in their own passphrase. Otherwise, just use
// the default passphrase. No confirmation of passphrase
func getPassphrase() (string, error) {
	if passphraseFilePath != "" {
		if _, err := os.Stat(passphraseFilePath); os.IsNotExist(err) {
			return "", errors.New(fmt.Sprintf("passphrase file not found at `%s`", passphraseFilePath))
		}
		dat, err := ioutil.ReadFile(passphraseFilePath)
		if err != nil {
			return "", err
		}
		pw := strings.ReplaceAll(string(dat), "\n", "")
		pw = strings.ReplaceAll(pw, "\t", "")
		pw = strings.TrimSpace(pw)
		return pw, nil
	} else if userProvidesPassphrase {
		fmt.Println("Enter wallet keystore passphrase:")
		pass, err := terminal.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			return "", err
		}
		return string(pass), nil
	} else {
		return c.DefaultPassphrase, nil
	}
}

// getPassphrase fetches the correct passphrase depending on if a file is available to
// read from or if the user wants to enter in their own passphrase. Otherwise, just use
// the default passphrase. Passphrase requires a confirmation
func getPassphraseWithConfirm() (string, error) {
	if passphraseFilePath != "" {
		if _, err := os.Stat(passphraseFilePath); os.IsNotExist(err) {
			return "", errors.New(fmt.Sprintf("passphrase file not found at `%s`", passphraseFilePath))
		}
		dat, err := ioutil.ReadFile(passphraseFilePath)
		if err != nil {
			return "", err
		}
		pw := strings.TrimSuffix(string(dat), "\n")
		return pw, nil
	} else if userProvidesPassphrase {
		fmt.Println("Enter passphrase:")
		pass, err := terminal.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			return "", err
		}
		fmt.Println("Repeat the passphrase:")
		repeatPass, err := terminal.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			return "", err
		}
		if string(repeatPass) != string(pass) {
			return "", errors.New("passphrase does not match")
		}
		fmt.Println("") // provide feedback when passphrase is entered.
		return string(repeatPass), nil
	} else {
		return c.DefaultPassphrase, nil
	}
}

func keysSub() []*cobra.Command {
	cmdList := &cobra.Command{
		Use:   "list",
		Short: "List all the local accounts",
		RunE: func(cmd *cobra.Command, args []string) error {
			if useLedgerWallet {
				ledger.ProcessAddressCommand()
				return nil
			}
			store.DescribeLocalAccounts()
			return nil
		},
	}

	cmdLocation := &cobra.Command{
		Use:   "location",
		Short: "Show where `itc` keeps accounts & their keys",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(store.DefaultLocation())
			return nil
		},
	}

	cmdAdd := &cobra.Command{
		Use:   "add <ACCOUNT_NAME>",
		Short: "Create a new keystore key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if store.DoesNamedAccountExist(args[0]) {
				return fmt.Errorf("account %s already exists", args[0])
			}
			passphrase, err := getPassphraseWithConfirm()
			if err != nil {
				return err
			}
			acc := account.Creation{
				Name:       args[0],
				Passphrase: passphrase,
			}
			if recoverFromMnemonic {
				fmt.Fprintf(os.Stderr, "deprecated method: use `./itc keys recover-from-mnemonic` instead.\n")
				fmt.Println("Enter mnemonic to recover keys from")
				scanner := bufio.NewScanner(os.Stdin)
				scanner.Scan()
				m := scanner.Text()
				if !bip39.IsMnemonicValid(m) {
					return mnemonic.InvalidMnemonic
				}
				acc.Mnemonic = m
			}
			if err := account.CreateNewLocalAccount(&acc); err != nil {
				return err
			}
			if !recoverFromMnemonic {
				color.Red(seedPhraseWarning)
				fmt.Println(acc.Mnemonic)
			}
			addr, _ := store.AddressFromAccountName(acc.Name)
			fmt.Printf("ITC Address: %s\n", addr)
			return nil
		},
	}
	cmdAdd.Flags().BoolVar(&recoverFromMnemonic, "recover", false, "create keys from a mnemonic")
	cmdAdd.Flags().BoolVar(&userProvidesPassphrase, "passphrase", false, ppPrompt)
	cmdAdd.Flags().StringVar(&passphraseFilePath, "passphrase-file", "", "path to a file containing the passphrase")

	cmdRemove := &cobra.Command{
		Use:   "remove <ACCOUNT_NAME>",
		Short: "Remove a key from the keystore",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := account.RemoveAccount(args[0]); err != nil {
				return err
			}
			return nil
		},
	}

	cmdMnemonic := &cobra.Command{
		Use:   "mnemonic",
		Short: "Compute the bip39 mnemonic for some input entropy",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(mnemonic.Generate())
			return nil
		},
	}

	cmdRecoverMnemonic := &cobra.Command{
		Use:   "recover-from-mnemonic [ACCOUNT_NAME]",
		Short: "Recover account from mnemonic",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if store.DoesNamedAccountExist(args[0]) {
				return fmt.Errorf("account %s already exists", args[0])
			}
			passphrase, err := getPassphraseWithConfirm()
			if err != nil {
				return err
			}
			acc := account.Creation{
				Name:       args[0],
				Passphrase: passphrase,
			}
			fmt.Println("Enter mnemonic to recover keys from")
			scanner := bufio.NewScanner(os.Stdin)
			scanner.Scan()
			m := scanner.Text()
			if !bip39.IsMnemonicValid(m) {
				return mnemonic.InvalidMnemonic
			}
			acc.Mnemonic = m
			if err := account.CreateNewLocalAccount(&acc); err != nil {
				return err
			}
			fmt.Println("Successfully recovered account from mnemonic!")
			addr, _ := store.AddressFromAccountName(acc.Name)
			fmt.Printf("ITC Address: %s\n", addr)
			return nil
		},
	}
	cmdRecoverMnemonic.Flags().BoolVar(&userProvidesPassphrase, "passphrase", false, ppPrompt)
	cmdRecoverMnemonic.Flags().StringVar(&passphraseFilePath, "passphrase-file", "", "path to a file containing the passphrase")

	cmdImportKS := &cobra.Command{
		Use:   "import-ks <KEYSTORE_FILE_PATH> [ACCOUNT_NAME]",
		Args:  cobra.RangeArgs(1, 2),
		Short: "Import an existing keystore key",
		RunE: func(cmd *cobra.Command, args []string) error {
			userName := ""
			if len(args) == 2 {
				userName = args[1]
			}
			passphrase, err := getPassphrase()
			if err != nil {
				return err
			}
			name, err := account.ImportKeyStore(args[0], userName, passphrase)
			if !quietImport && err == nil {
				fmt.Printf("Imported keystore given account alias of `%s`\n", name)
				addr, _ := store.AddressFromAccountName(name)
				fmt.Printf("ITC Address: %s\n", addr)
			}
			return err
		},
	}
	cmdImportKS.Flags().BoolVar(&userProvidesPassphrase, "passphrase", false, ppPrompt)
	cmdImportKS.Flags().StringVar(&passphraseFilePath, "passphrase-file", "", "path to a file containing the passphrase")
	cmdImportKS.Flags().BoolVar(&quietImport, "quiet", false, "do not print out imported account name")

	cmdImportPK := &cobra.Command{
		Use:   "import-private-key <secp256k1_PRIVATE_KEY> [ACCOUNT_NAME]",
		Short: "Import an existing keystore key (only accept secp256k1 private keys)",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			userName := ""
			if len(args) == 2 {
				userName = args[1]
			}
			passphrase, err := getPassphrase()
			if err != nil {
				return err
			}
			name, err := account.ImportFromPrivateKey(args[0], userName, passphrase)
			if !quietImport && err == nil {
				fmt.Printf("Imported keystore given account alias of `%s`\n", name)
				addr, _ := store.AddressFromAccountName(name)
				fmt.Printf("ITC Address: %s\n", addr)
			}
			return err
		},
	}
	cmdImportPK.Flags().BoolVar(&userProvidesPassphrase, "passphrase", false, ppPrompt)
	cmdImportPK.Flags().BoolVar(&quietImport, "quiet", false, "do not print out imported account name")

	cmdExportPK := &cobra.Command{
		Use:     "export-private-key <ACCOUNT_ADDRESS>",
		Short:   "Export the secp256k1 private key",
		Args:    cobra.ExactArgs(1),
		PreRunE: validateAddress,
		RunE: func(cmd *cobra.Command, args []string) error {
			passphrase, err := getPassphrase()
			if err != nil {
				return err
			}
			return account.ExportPrivateKey(addr.address, passphrase)
		},
	}
	cmdExportPK.Flags().BoolVar(&userProvidesPassphrase, "passphrase", false, ppPrompt)
	cmdExportPK.Flags().StringVar(&passphraseFilePath, "passphrase-file", "", "path to a file containing the passphrase")

	cmdExportKS := &cobra.Command{
		Use:     "export-ks <ACCOUNT_ADDRESS> <OUTPUT_DIRECTORY>",
		Short:   "Export the keystore file contents",
		Args:    cobra.ExactArgs(2),
		PreRunE: validateAddress,
		RunE: func(cmd *cobra.Command, args []string) error {
			passphrase, err := getPassphrase()
			if err != nil {
				return err
			}
			file, e := account.ExportKeystore(addr.address, args[1], passphrase)
			if file != "" {
				fmt.Println("Exported keystore to", file)
			}
			return e
		},
	}
	cmdExportKS.Flags().BoolVar(&userProvidesPassphrase, "passphrase", false, ppPrompt)
	cmdExportKS.Flags().StringVar(&passphraseFilePath, "passphrase-file", "", "path to a file containing the passphrase")

	cmdGenerateBlsKey := &cobra.Command{
		Use:   "generate-bls-key",
		Short: "Generate bls keys then encrypt and save the private key with a requested passphrase",
		RunE: func(cmd *cobra.Command, args []string) error {
			passphrase, err := getPassphraseWithConfirm()
			if err != nil {
				return err
			}

			blsKey := &keys.BlsKey{
				Passphrase: passphrase,
				FilePath:   blsFilePath,
			}

			return keys.GenBlsKey(blsKey)
		},
	}
	cmdGenerateBlsKey.Flags().StringVar(&blsFilePath, "bls-file-path", "",
		"absolute path of where to save encrypted bls private key")
	cmdGenerateBlsKey.Flags().BoolVar(&userProvidesPassphrase, "passphrase", false, ppPrompt)
	cmdGenerateBlsKey.Flags().StringVar(&passphraseFilePath, "passphrase-file", "", "path to a file containing the passphrase")

	cmdGenerateMultiBlsKeys := &cobra.Command{
		Use:   "generate-bls-keys",
		Short: "Generates multiple bls keys for a given shard network configuration and then encrypts and saves the private key with a requested passphrase",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validation.ValidateNodeConnection(node); err != nil {
				fmt.Fprintf(os.Stderr, "Cannot connect to node %v, using intelchain mainnet endpoint %v\n",
					node, defaultMainnetEndpoint)
				node = defaultMainnetEndpoint
			}
			blsKeys := []*keys.BlsKey{}

			for i := uint32(0); i < blsCount; i++ {
				keyFilePath := blsFilePath
				if blsFilePath != "" {
					fmt.Printf("Enter absolute path for key #%d:\n", i+1)
					fmt.Scanln(&keyFilePath)
				}

				passphrase, err := getPassphraseWithConfirm()
				if err != nil {
					return err
				}

				blsKey := &keys.BlsKey{
					Passphrase: passphrase,
					FilePath:   keyFilePath,
				}
				blsKeys = append(blsKeys, blsKey)
			}

			return keys.GenMultiBlsKeys(blsKeys, node, blsShardID)
		},
	}
	cmdGenerateMultiBlsKeys.Flags().StringVar(&blsFilePath, "bls-file-path", "",
		"absolute path of where to save encrypted bls private keys")
	cmdGenerateMultiBlsKeys.Flags().BoolVar(&userProvidesPassphrase, "passphrase", false, ppPrompt)
	cmdGenerateMultiBlsKeys.Flags().StringVar(&passphraseFilePath, "passphrase-file", "", "path to a file containing the passphrase")
	cmdGenerateMultiBlsKeys.Flags().Uint32Var(&blsShardID, "shard", 0, "which shard to create bls keys for")
	cmdGenerateMultiBlsKeys.Flags().Uint32Var(&blsCount, "count", 1, "how many bls keys to generate")

	cmdRecoverBlsKey := &cobra.Command{
		Use:   "recover-bls-key <ABSOLUTE_PATH_BLS_KEY>",
		Short: "Recover bls keys from an encrypted bls key file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			passphrase, err := getPassphrase()
			if err != nil {
				return err
			}
			return keys.RecoverBlsKeyFromFile(passphrase, args[0])
		},
	}
	cmdRecoverBlsKey.Flags().BoolVar(&userProvidesPassphrase, "passphrase", false, ppPrompt)
	cmdRecoverBlsKey.Flags().StringVar(&passphraseFilePath, "passphrase-file", "", "path to a file containing the passphrase")

	cmdSaveBlsKey := &cobra.Command{
		Use:   "save-bls-key <PRIVATE_BLS_KEY>",
		Short: "Encrypt and save the bls private key with a requested passphrase",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			passphrase, err := getPassphraseWithConfirm()
			if err != nil {
				return err
			}
			return keys.SaveBlsKey(passphrase, blsFilePath, args[0])
		},
	}
	cmdSaveBlsKey.Flags().StringVar(&blsFilePath, "bls-file-path", "",
		"absolute path of where to save encrypted bls private key")
	cmdSaveBlsKey.Flags().BoolVar(&userProvidesPassphrase, "passphrase", false, ppPrompt)
	cmdSaveBlsKey.Flags().StringVar(&passphraseFilePath, "passphrase-file", "", "path to a file containing the passphrase")

	GetPublicBlsKey := &cobra.Command{
		Use:   "get-public-bls-key <PRIVATE_BLS_KEY>",
		Short: "Get the public bls key associated with the provided private bls key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return keys.GetPublicBlsKey(args[0])
		},
	}

	cmdCheckPassphrase := &cobra.Command{
		Use:     "check-passphrase <ACCOUNT_ADDRESS>",
		Short:   "Check if passphrase for given account is valid.",
		Args:    cobra.ExactArgs(1),
		PreRunE: validateAddress,
		RunE: func(cmd *cobra.Command, args []string) error {
			userProvidesPassphrase = true
			passphrase, err := getPassphrase()
			if err != nil {
				return err
			}
			ok, err := account.VerifyPassphrase(args[0], passphrase)
			if ok {
				fmt.Println("Valid passphrase")
				return nil
			}
			if err != nil {
				return err
			}
			return fmt.Errorf("invalid passphrase")
		},
	}
	cmdCheckPassphrase.Flags().StringVar(&passphraseFilePath, "passphrase-file", "", "path to a file containing the passphrase")

	return []*cobra.Command{cmdList, cmdLocation, cmdAdd, cmdRemove, cmdMnemonic, cmdRecoverMnemonic,
		cmdImportKS, cmdImportPK, cmdExportKS, cmdExportPK, cmdCheckPassphrase,
		cmdGenerateBlsKey, cmdGenerateMultiBlsKeys, cmdRecoverBlsKey, cmdSaveBlsKey, GetPublicBlsKey}
}

func init() {
	cmdKeys := &cobra.Command{
		Use:   "keys",
		Short: "Add or view local private keys",
		Long:  "Manage your local keys",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Help()
			return nil
		},
	}

	cmdKeys.AddCommand(keysSub()...)
	RootCmd.AddCommand(cmdKeys)
}
