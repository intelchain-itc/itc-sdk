package cmd

import (
	"fmt"

	"github.com/fatih/color"
)

const (
	itcDocsDir             = "itc-docs"
	defaultNodeAddr        = "http://localhost:9500"
	defaultRpcPrefix       = "itc"
	defaultMainnetEndpoint = "https://testnet.intelchain.network/"
)

var (
	g           = color.New(color.FgGreen).SprintFunc()
	cookbookDoc = fmt.Sprintf(`
Cookbook of Usage

Note:

1) Every subcommand recognizes a '--help' flag
2) If a passphrase is used by a subcommand, one can enter their own passphrase interactively
   with the --passphrase option. Alternatively, one can pass their own passphrase via a file
   using the --passphrase-file option. If no passphrase option is selected, the default
   passphrase of '' is used.
3) These examples use Shard 0 of [NETWORK] as argument for --node

Examples:

%s
./itc --node=[NODE] balances <SOME_ITC_ADDRESS>

%s
./itc --node=[NODE] blockchain transaction-by-hash <SOME_TX_HASH>

%s
./itc keys list

%s
./itc --node=[NODE] transfer \
    --from <SOME_ITC_ADDRESS> --to <SOME_ITC_ADDRESS> \
    --from-shard 0 --to-shard 1 --amount 200 --passphrase

%s
./itc --node=[NODE] transfer --file <PATH_TO_JSON_FILE>
Check README for details on json file format.

%s
./itc --node=[NODE] blockchain transaction-receipt <SOME_TX_HASH>

%s
./itc keys recover-from-mnemonic <ACCOUNT_NAME> --passphrase

%s
./itc keys import-ks <PATH_TO_KEYSTORE_JSON>

%s
./itc keys import-private-key <secp256k1_PRIVATE_KEY>

%s
./itc keys export-private-key <ACCOUNT_ADDRESS> --passphrase

%s
./itc keys generate-bls-key --bls-file-path <PATH_FOR_BLS_KEY_FILE>

%s
./itc --node=[NODE] staking create-validator --amount 10 --validator-addr <SOME_ITC_ADDRESS> \
    --bls-pubkeys <BLS_KEY_1>,<BLS_KEY_2>,<BLS_KEY_3> \
    --identity foo --details bar --name baz --max-change-rate 0.1 --max-rate 0.1 --max-total-delegation 10 \
    --min-self-delegation 10 --rate 0.1 --security-contact Leo  --website intelchain.org --passphrase

%s
./itc --node=[NODE] staking edit-validator \
    --validator-addr <SOME_ITC_ADDRESS> --identity foo --details bar \
    --name baz --security-contact EK --website intelchain.org \
    --min-self-delegation 0 --max-total-delegation 10 --rate 0.1\
    --add-bls-key <SOME_BLS_KEY> --remove-bls-key <OTHER_BLS_KEY> --passphrase

%s
./itc --node=[NODE] staking delegate \
    --delegator-addr <SOME_ITC_ADDRESS> --validator-addr <VALIDATOR_ITC_ADDRESS> \
    --amount 10 --passphrase

%s
./itc --node=[NODE] staking undelegate \
    --delegator-addr <SOME_ITC_ADDRESS> --validator-addr <VALIDATOR_ITC_ADDRESS> \
    --amount 10 --passphrase

%s
./itc --node=[NODE] staking collect-rewards \
    --delegator-addr <SOME_ITC_ADDRESS> --passphrase

%s
./itc --node=[NODE] blockchain validator elected

%s
./itc --node=[NODE] blockchain utility-metrics

%s
./itc --node=[NODE] failures staking

%s
./itc --node=[NODE] utility shard-for-bls <BLS_PUBLIC_KEY>

%s
./itc governance vote-proposal --space=[intelchain-mainnet.eth] \
	--proposal=<PROPOSAL_IPFS_HASH> --proposal-type=[single-choice] \
	--choice=<VOTING_CHOICE(S)> --app=[APP] --key=<ACCOUNT_ADDRESS_OR_NAME>
PS: key must first use (itc keys import-private-key) to import

%s
./itc command --net=testnet
`,
		g("1.  Check account balance on given chain"),
		g("2.  Check sent transaction"),
		g("3.  List local account keys"),
		g("4.  Sending a transaction (waits 40 seconds for transaction confirmation)"),
		g("5.  Sending a batch of transactions as dictated from a file (the `--dry-run` options still apply)"),
		g("6.  Check a completed transaction receipt"),
		g("7.  Import an account using the mnemonic. Prompts the user to give the mnemonic."),
		g("8.  Import an existing keystore file"),
		g("9.  Import a keystore file using a secp256k1 private key"),
		g("10. Export a keystore file's secp256k1 private key"),
		g("11. Generate a BLS key then encrypt and save the private key to the specified location."),
		g("12. Create a new validator with a list of BLS keys"),
		g("13. Edit an existing validator"),
		g("14. Delegate an amount to a validator"),
		g("15. Undelegate to a validator"),
		g("16. Collect block rewards as a delegator"),
		g("17. Check elected validators"),
		g("18. Get current staking utility metrics"),
		g("19. Check in-memory record of failed staking transactions"),
		g("20. Check which shard your BLS public key would be assigned to as a validator"),
		g("21. Vote on a governance proposal on https://snapshot.org"),
		g("22. Enter console"),
	)
)
