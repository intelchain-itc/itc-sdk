package cmd

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rlp"
	bls_core "github.com/intelchain-itc/bls/ffi/go/bls"
	"github.com/intelchain-itc/intelchain/accounts"
	"github.com/intelchain-itc/intelchain/accounts/keystore"
	"github.com/intelchain-itc/intelchain/common/denominations"
	"github.com/intelchain-itc/intelchain/core"
	"github.com/intelchain-itc/intelchain/crypto/bls"
	"github.com/intelchain-itc/intelchain/numeric"
	"github.com/intelchain-itc/intelchain/shard"
	"github.com/intelchain-itc/intelchain/staking/effective"
	staking "github.com/intelchain-itc/intelchain/staking/types"
	"github.com/intelchain-itc/itc-sdk/pkg/address"
	"github.com/intelchain-itc/itc-sdk/pkg/common"
	"github.com/intelchain-itc/itc-sdk/pkg/keys"
	"github.com/intelchain-itc/itc-sdk/pkg/ledger"
	"github.com/intelchain-itc/itc-sdk/pkg/rpc"
	"github.com/intelchain-itc/itc-sdk/pkg/store"
	"github.com/intelchain-itc/itc-sdk/pkg/transaction"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

const (
	blsPubKeySize            = 48
	MaxNameLength            = 140
	MaxIdentityLength        = 140
	MaxWebsiteLength         = 140
	MaxSecurityContactLength = 140
	MaxDetailsLength         = 280
)

var (
	validatorName             string
	validatorIdentity         string
	validatorWebsite          string
	validatorSecurityContact  string
	validatorDetails          string
	commisionRateStr          string
	commisionMaxRateStr       string
	commisionMaxChangeRateStr string
	slotKeyToRemove           string
	slotKeyToAdd              string
	minSelfDelegation         string
	maxTotalDelegation        string
	stakingBlsPubKeys         []string
	blsPubKeyDir              string
	delegatorAddress          itcAddress
	validatorAddress          itcAddress
	stakingAmount             string
	active                    string
	itcAsDec                  = numeric.NewDec(denominations.Itc)
	nanoAsDec                 = numeric.NewDec(denominations.Ticks)
)

var (
	errSelfDelegationTooSmall          = errors.New("amount can not be less than min-self-delegation")
	errSelfDelegationTooLarge          = errors.New("amount can not be greater than max-total-delegation")
	errInvalidTotalDelegation          = errors.New("max-total-delegation can not be bigger than max-total-delegation")
	errMinSelfDelegationTooSmall       = errors.New("min-self-delegation can not be less than 1 ITC")
	errMaxTotalDelegationTooSmall      = errors.New("max-total-delegation can not be less than 1 ITC")
	errInvalidMaxTotalDelegation       = errors.New("max-total-delegation can not be less than min-self-delegation")
	errCommissionRateTooLarge          = errors.New("rate can not be greater than max-commission-rate")
	errChangeRateTooLarge              = errors.New("max-change-rate can not be greater than max-commission-rate")
	errInvalidCommissionRateLow        = errors.New("rate can not be less than 0, must be between 0 and 1")
	errInvalidCommissionRateHigh       = errors.New("rate can not be greater than 1, must be between 0 and 1")
	errInvalidChangeRateLow            = errors.New("max-change-rate can not be less than 0, must be between 0 and 1")
	errInvalidChangeRateHigh           = errors.New("max-change-rate can not be greater than 1, must be between 0 and 1")
	errInvalidMaxRateLow               = errors.New("max-commission-rate can not be less than 0, must be between 0 and 1")
	errInvalidMaxRateHigh              = errors.New("max-commission-rate can not be greater than 1, must be between 0 and 1")
	errInvalidDescFieldName            = errors.New("exceeds maximum length of 140 characters for name")
	errInvalidDescFieldIdentity        = errors.New("exceeds maximum length of 140 characters for identity")
	errInvalidDescFieldWebsite         = errors.New("exceeds maximum length of 140 characters for website")
	errInvalidDescFieldSecurityContact = errors.New("exceeds maximum length of 140 characters for security-contact")
	errInvalidDescFieldDetails         = errors.New("exceeds maximum length of 280 characters for details")
	errNegativeAmount                  = errors.New("amount can not be negative")
)

func createStakingTransaction(nonce uint64, f staking.StakeMsgFulfiller) (*staking.StakingTransaction, error) {
	gPrice, err := common.NewDecFromString(gasPrice)
	if err != nil {
		return nil, err
	}
	gPrice = gPrice.Mul(nanoAsDec)
	directive, payload := f()
	data, err := rlp.EncodeToBytes(payload)
	if err != nil {
		return nil, err
	}
	var gLimit uint64
	if gasLimit == "" {
		isCreateValidator := directive == staking.DirectiveCreateValidator
		gLimit, err = core.IntrinsicGas(data, false, true, true, isCreateValidator)
		if err != nil {
			return nil, err
		}
	} else {
		tempLimit, e := strconv.ParseInt(gasLimit, 10, 64)
		if e != nil {
			return nil, e
		}
		gLimit = uint64(tempLimit)
	}

	stakingTx, err := staking.NewStakingTransaction(nonce, gLimit, gPrice.TruncateInt(), f)
	return stakingTx, err
}

func handleStakingTransaction(
	stakingTx *staking.StakingTransaction, networkHandler *rpc.HTTPMessenger, signerAddress itcAddress,
) error {
	var (
		ks     *keystore.KeyStore
		acct   *accounts.Account
		signed *staking.StakingTransaction
		err    error
	)

	from := signerAddress.String()

	if useLedgerWallet {
		signerAddr := ""
		signed, signerAddr, err = ledger.SignStakingTx(stakingTx, chainName.chainID.Value)
		if err != nil {
			return err
		}

		if strings.Compare(signerAddr, from) != 0 {
			return errors.New("error : delegator address doesn't match with ledger hardware addresss")
		}
	} else {
		ks, acct, err = store.UnlockedKeystore(from, passphrase)
		if err != nil {
			return err
		}
		signed, err = ks.SignStakingTx(*acct, stakingTx, chainName.chainID.Value)
	}

	if err != nil {
		return err
	}

	enc, err := rlp.EncodeToBytes(signed)
	if err != nil {
		return err
	}

	hexSignature := hexutil.Encode(enc)
	reply, err := networkHandler.SendRPC(rpc.Method.SendRawStakingTransaction, []interface{}{hexSignature})
	if err != nil {
		return err
	}
	r, _ := reply["result"].(string)
	if timeout > 0 {
		if err := confirmTx(networkHandler, timeout, r); err != nil {
			fmt.Println(fmt.Sprintf(`{"transaction-hash":"%s"}`, r))
			return err
		}
	} else {
		fmt.Println(fmt.Sprintf(`{"transaction-receipt":"%s"}`, r))
	}
	return nil
}

func confirmTx(networkHandler *rpc.HTTPMessenger, confirmWaitTime uint32, txHash string) error {
	start := int(confirmWaitTime)
	for {
		r, _ := networkHandler.SendRPC(rpc.Method.GetTransactionReceipt, []interface{}{txHash})
		if r["result"] != nil {
			fmt.Println(common.ToJSONUnsafe(r, true))
			return nil
		}
		if start < 0 {
			transactionErrors, _ := transaction.GetError(txHash, networkHandler)
			for _, txError := range transactionErrors {
				fmt.Println(txError.Error().Error())
			}
			fmt.Println("Try increasing the `timeout` or look for the transaction receipt with `itc blockchain transaction-receipt <txHash>`")
			return fmt.Errorf("could not confirm %s even after %d seconds", txHash, confirmWaitTime)
		}
		transactionErrors, _ := transaction.GetError(txHash, networkHandler)
		if len(transactionErrors) > 0 {
			for _, txError := range transactionErrors {
				fmt.Println(txError.Error().Error())
			}
			return fmt.Errorf("staking transaction error")
		}
		time.Sleep(time.Second * 2)
		start = start - 2
	}
}

func delegationAmountSanityCheck(minSelfDelegation *numeric.Dec, maxTotalDelegation *numeric.Dec, amount *numeric.Dec) error {
	// MinSelfDelegation must be >= 1 ITC
	if minSelfDelegation != nil && minSelfDelegation.LT(itcAsDec) {
		return errMinSelfDelegationTooSmall
	}

	// MaxTotalDelegation must be a
	if maxTotalDelegation != nil && maxTotalDelegation.LT(itcAsDec) {
		return errMaxTotalDelegationTooSmall
	}

	// MaxTotalDelegation must not be less than MinSelfDelegation
	if minSelfDelegation != nil && maxTotalDelegation != nil &&
		maxTotalDelegation.LT(*minSelfDelegation) {
		return errInvalidMaxTotalDelegation
	}

	// Amount must be >= MinSelfDelegation
	if minSelfDelegation != nil && maxTotalDelegation != nil && amount != nil {
		if amount.LT(*minSelfDelegation) {
			return errSelfDelegationTooSmall
		}
		if amount.GT(*maxTotalDelegation) {
			return errSelfDelegationTooLarge
		}
	}

	return nil
}

func rateSanityCheck(rate numeric.Dec, maxRate numeric.Dec, maxChangeRate numeric.Dec) error {
	hundredPercent := numeric.NewDec(1)
	zeroPercent := numeric.NewDec(0)

	if rate.LT(zeroPercent) {
		return errInvalidCommissionRateLow
	}

	if rate.GT(hundredPercent) {
		return errInvalidCommissionRateHigh
	}

	if maxRate.LT(zeroPercent) {
		return errInvalidMaxRateLow
	}

	if maxRate.GT(hundredPercent) {
		return errInvalidMaxRateHigh
	}

	if maxChangeRate.LT(zeroPercent) {
		return errInvalidChangeRateLow
	}

	if maxChangeRate.GT(hundredPercent) {
		return errInvalidChangeRateHigh
	}

	if rate.GT(maxRate) {
		return errCommissionRateTooLarge
	}

	if maxChangeRate.GT(maxRate) {
		return errChangeRateTooLarge
	}

	return nil
}

func ensureLength(d staking.Description) (staking.Description, error) {
	if len(d.Name) > MaxNameLength {
		return d, errInvalidDescFieldName
	}
	if len(d.Identity) > MaxIdentityLength {
		return d, errInvalidDescFieldIdentity
	}
	if len(d.Website) > MaxWebsiteLength {
		return d, errInvalidDescFieldWebsite
	}
	if len(d.SecurityContact) > MaxSecurityContactLength {
		return d, errInvalidDescFieldSecurityContact
	}
	if len(d.Details) > MaxDetailsLength {
		return d, errInvalidDescFieldDetails
	}
	return d, nil
}

func assertValidOptionStringInputForNonNumbers(input string) error {
	if input != "" && strings.HasPrefix(input, "-") {
		return fmt.Errorf("invalid or missing option")
	}
	return nil
}

func validateBlsKeyInput(cmd *cobra.Command, args []string) error {
	if err := assertValidOptionStringInputForNonNumbers(slotKeyToAdd); err != nil {
		return errors.WithMessage(err, "BLS key to add error")
	}
	if err := assertValidOptionStringInputForNonNumbers(slotKeyToRemove); err != nil {
		return errors.WithMessage(err, "BLS key to remove error")
	}
	for _, str := range stakingBlsPubKeys {
		if err := assertValidOptionStringInputForNonNumbers(str); err != nil {
			return errors.WithMessage(err, "BLS key to create validator error")
		}
	}
	return nil
}

func stakingSubCommands() []*cobra.Command {

	subCmdNewValidator := &cobra.Command{
		Use:   "create-validator",
		Short: "create a new validator",
		Args:  cobra.ExactArgs(0),
		Long: `
Create a new validator"
`,
		PreRunE: validateBlsKeyInput,
		RunE: func(cmd *cobra.Command, args []string) error {
			networkHandler, err := handlerForShard(0, node)
			if err != nil {
				return err
			}

			commisionRate, err := common.NewDecFromString(commisionRateStr)
			if err != nil {
				return err
			}

			commisionMaxRate, err := common.NewDecFromString(commisionMaxRateStr)
			if err != nil {
				return err
			}

			commisionMaxChangeRate, err := common.NewDecFromString(commisionMaxChangeRateStr)
			if err != nil {
				return err
			}

			blsPubKeys := make([]bls.SerializedPublicKey, len(stakingBlsPubKeys))
			for i := 0; i < len(stakingBlsPubKeys); i++ {
				blsPubKey := new(bls_core.PublicKey)
				err = blsPubKey.DeserializeHexStr(strings.TrimPrefix(stakingBlsPubKeys[i], "0x"))
				if err != nil {
					return err
				}

				blsPubKeys[i].FromLibBLSPublicKey(blsPubKey)
			}

			blsSigs, err := keys.VerifyBLSKeys(stakingBlsPubKeys, blsPubKeyDir)
			if err != nil {
				return err
			}

			amt, e0 := common.NewDecFromString(stakingAmount)
			minSelfDel, e1 := common.NewDecFromString(minSelfDelegation)
			maxTotalDel, e2 := common.NewDecFromString(maxTotalDelegation)

			if e0 != nil {
				return e0
			}
			if e1 != nil {
				return e1
			}
			if e2 != nil {
				return e2
			}

			amt = amt.Mul(itcAsDec)
			minSelfDel = minSelfDel.Mul(itcAsDec)
			maxTotalDel = maxTotalDel.Mul(itcAsDec)

			err = delegationAmountSanityCheck(&minSelfDel, &maxTotalDel, &amt)
			if err != nil {
				return err
			}

			err = rateSanityCheck(commisionRate, commisionMaxRate, commisionMaxChangeRate)
			if err != nil {
				return err
			}

			desc, err := ensureLength(staking.Description{
				validatorName,
				validatorIdentity,
				validatorWebsite,
				validatorSecurityContact,
				validatorDetails,
			})
			if err != nil {
				return err
			}

			delegateStakePayloadMaker := func() (staking.Directive, interface{}) {
				return staking.DirectiveCreateValidator, staking.CreateValidator{
					address.Parse(validatorAddress.String()),
					desc,
					staking.CommissionRates{
						commisionRate,
						commisionMaxRate,
						commisionMaxChangeRate},
					minSelfDel.RoundInt(),
					maxTotalDel.RoundInt(),
					blsPubKeys,
					blsSigs,
					amt.RoundInt(),
				}
			}

			nonce, err := getNonce(validatorAddress.String(), networkHandler)
			if err != nil {
				return err
			}
			stakingTx, err := createStakingTransaction(nonce, delegateStakePayloadMaker)
			if err != nil {
				return err
			}

			passphrase, err = getPassphrase()
			if err != nil {
				return err
			}

			err = handleStakingTransaction(stakingTx, networkHandler, validatorAddress)
			if err != nil {
				return err
			}
			return nil
		},
	}

	subCmdNewValidator.Flags().BoolVar(&trueNonce, "true-nonce", false, "send transaction with on-chain nonce")
	subCmdNewValidator.Flags().StringVar(&validatorName, "name", "", "validator's name")
	subCmdNewValidator.Flags().StringVar(&validatorIdentity, "identity", "", "validator's identity")
	subCmdNewValidator.Flags().StringVar(&validatorWebsite, "website", "", "validator's website")
	subCmdNewValidator.Flags().StringVar(&validatorSecurityContact, "security-contact", "", "validator's security contact")
	subCmdNewValidator.Flags().StringVar(&validatorDetails, "details", "", "validator's details")
	subCmdNewValidator.Flags().StringVar(&commisionRateStr, "rate", "", "commission rate")
	subCmdNewValidator.Flags().StringVar(&commisionMaxRateStr, "max-rate", "", "commision max rate")
	subCmdNewValidator.Flags().StringVar(&commisionMaxChangeRateStr, "max-change-rate", "", "commission max change amount")
	subCmdNewValidator.Flags().StringVar(&minSelfDelegation, "min-self-delegation", "0.0", "minimal self delegation")
	subCmdNewValidator.Flags().StringVar(
		&maxTotalDelegation, "max-total-delegation", "0.0", "maximal total delegation",
	)
	subCmdNewValidator.Flags().Var(
		&validatorAddress, "validator-addr", "validator's staking address",
	)
	subCmdNewValidator.Flags().StringSliceVar(
		&stakingBlsPubKeys, "bls-pubkeys",
		[]string{}, "validator's list of public BLS key addresses",
	)
	subCmdNewValidator.Flags().StringVar(&blsPubKeyDir, "bls-pubkeys-dir", "", "directory to bls pubkeys storing pub.key, pub.pass files")
	subCmdNewValidator.Flags().StringVar(&stakingAmount, "amount", "0.0", "staking amount")
	subCmdNewValidator.Flags().StringVar(&gasPrice, "gas-price", "100", "gas price to pay")
	subCmdNewValidator.Flags().StringVar(&gasLimit, "gas-limit", "", "gas limit")
	subCmdNewValidator.Flags().StringVar(&inputNonce, "nonce", "", "set nonce for transaction")
	subCmdNewValidator.Flags().StringVar(&targetChain, "chain-id", "", "what chain ID to target")
	subCmdNewValidator.Flags().Uint32Var(
		&timeout, "timeout",
		defaultTimeout, "set timeout in seconds. Set to 0 to not wait for tx confirm",
	)
	subCmdNewValidator.Flags().BoolVar(&userProvidesPassphrase, "passphrase", false, ppPrompt)
	subCmdNewValidator.Flags().StringVar(
		&passphraseFilePath, "passphrase-file", "", "path to a file containing the passphrase",
	)

	for _, flagName := range [...]string{
		"name", "identity", "website", "security-contact",
		"details", "rate", "max-rate", "max-change-rate",
		"min-self-delegation", "max-total-delegation",
		"validator-addr", "bls-pubkeys", "amount"} {
		subCmdNewValidator.MarkFlagRequired(flagName)
	}

	subCmdEditValidator := &cobra.Command{
		Use:     "edit-validator",
		Short:   "edit a validator",
		Long:    "Edit an existing validator",
		Args:    cobra.ExactArgs(0),
		PreRunE: validateBlsKeyInput,
		RunE: func(cmd *cobra.Command, args []string) error {
			networkHandler, err := handlerForShard(shard.BeaconChainShardID, node)
			if err != nil {
				return err
			}

			var commisionRate *numeric.Dec
			if commisionRateStr != "" {
				cRate, err := common.NewDecFromString(commisionRateStr)
				if err != nil {
					return err
				}
				commisionRate = &cRate
			}

			var shardPubKeyRemove *bls.SerializedPublicKey
			if slotKeyToRemove != "" {
				blsKey := new(bls_core.PublicKey)
				err = blsKey.DeserializeHexStr(strings.TrimPrefix(slotKeyToRemove, "0x"))
				if err != nil {
					return err
				}
				shardKey := bls.SerializedPublicKey{}
				shardKey.FromLibBLSPublicKey(blsKey)
				shardPubKeyRemove = &shardKey
			}

			var shardPubKeyAdd *bls.SerializedPublicKey
			var sigBls *bls.SerializedSignature
			if slotKeyToAdd != "" {
				blsKey := new(bls_core.PublicKey)
				err = blsKey.DeserializeHexStr(strings.TrimPrefix(slotKeyToAdd, "0x"))
				if err != nil {
					return err
				}

				shardKey := bls.SerializedPublicKey{}
				shardKey.FromLibBLSPublicKey(blsKey)
				shardPubKeyAdd = &shardKey

				sig, err := keys.VerifyBLS(strings.TrimPrefix(slotKeyToAdd, "0x"), blsPubKeyDir)
				if err != nil {
					return err
				}
				sigBls = &sig
			}

			var minSelfDel *numeric.Dec
			var mSelDel *big.Int
			if minSelfDelegation != "" {
				amount, err := common.NewDecFromString(minSelfDelegation)
				if err != nil {
					return err
				}
				amount = amount.Mul(itcAsDec)
				minSelfDel = &amount
				mSelDel = amount.RoundInt()
			}

			var maxTotalDel *numeric.Dec
			var mTotalDel *big.Int
			if maxTotalDelegation != "" {
				amount, err := common.NewDecFromString(maxTotalDelegation)
				if err != nil {
					return err
				}
				amount = amount.Mul(itcAsDec)
				maxTotalDel = &amount
				mTotalDel = amount.RoundInt()
			}

			err = delegationAmountSanityCheck(minSelfDel, maxTotalDel, nil)
			if err != nil {
				return err
			}

			desc, err := ensureLength(staking.Description{
				validatorName,
				validatorIdentity,
				validatorWebsite,
				validatorSecurityContact,
				validatorDetails,
			})
			if err != nil {
				return err
			}

			EposStat := effective.Nil
			if active != "" {
				active, err := strconv.ParseBool(active)
				if err != nil {
					return err
				}
				if active {
					EposStat = effective.Active
				} else {
					EposStat = effective.Inactive
				}
			}

			delegateStakePayloadMaker := func() (staking.Directive, interface{}) {
				return staking.DirectiveEditValidator, staking.EditValidator{
					address.Parse(validatorAddress.String()),
					desc,
					commisionRate,
					mSelDel,
					mTotalDel,
					shardPubKeyRemove,
					shardPubKeyAdd,
					sigBls,
					EposStat,
				}
			}

			nonce, err := getNonce(validatorAddress.String(), networkHandler)
			if err != nil {
				return err
			}
			stakingTx, err := createStakingTransaction(nonce, delegateStakePayloadMaker)
			if err != nil {
				return err
			}

			passphrase, err = getPassphrase()
			if err != nil {
				return err
			}

			err = handleStakingTransaction(stakingTx, networkHandler, validatorAddress)
			if err != nil {
				return err
			}
			return nil
		},
	}

	subCmdEditValidator.Flags().BoolVar(&trueNonce, "true-nonce", false, "send transaction with on-chain nonce")
	subCmdEditValidator.Flags().StringVar(&validatorName, "name", "", "validator's name")
	subCmdEditValidator.Flags().StringVar(&validatorIdentity, "identity", "", "validator's identity")
	subCmdEditValidator.Flags().StringVar(&validatorWebsite, "website", "", "validator's website")
	subCmdEditValidator.Flags().StringVar(&validatorSecurityContact, "security-contact", "", "validator's security contact")
	subCmdEditValidator.Flags().StringVar(&validatorDetails, "details", "", "validator's details")
	subCmdEditValidator.Flags().StringVar(&commisionRateStr, "rate", "", "commission rate")
	subCmdEditValidator.Flags().StringVar(&blsPubKeyDir, "bls-pubkeys-dir", "", "directory to bls pubkeys storing pub.key, pub.pass files")
	subCmdEditValidator.Flags().StringVar(&minSelfDelegation, "min-self-delegation", "", "minimal self delegation")
	subCmdEditValidator.Flags().StringVar(&maxTotalDelegation, "max-total-delegation", "", "maximal total delegation")
	subCmdEditValidator.Flags().Var(&validatorAddress, "validator-addr", "validator's staking address")
	subCmdEditValidator.Flags().StringVar(&slotKeyToAdd, "add-bls-key", "", "add BLS pubkey to slot")
	subCmdEditValidator.Flags().StringVar(&slotKeyToRemove, "remove-bls-key", "", "remove BLS pubkey from slot")
	subCmdEditValidator.Flags().StringVar(&active, "active", "", "validator active true/false")

	subCmdEditValidator.Flags().StringVar(&gasPrice, "gas-price", "100", "gas price to pay")
	subCmdEditValidator.Flags().StringVar(&gasLimit, "gas-limit", "", "gas limit")
	subCmdEditValidator.Flags().StringVar(&inputNonce, "nonce", "", "set nonce for transaction")
	subCmdEditValidator.Flags().StringVar(&targetChain, "chain-id", "", "what chain ID to target")
	subCmdEditValidator.Flags().Uint32Var(&timeout, "timeout", defaultTimeout, "set timeout in seconds. Set to 0 to not wait for tx confirm")
	subCmdEditValidator.Flags().BoolVar(&userProvidesPassphrase, "passphrase", false, ppPrompt)
	subCmdEditValidator.Flags().StringVar(&passphraseFilePath, "passphrase-file", "", "path to a file containing the passphrase")

	subCmdEditValidator.MarkFlagRequired("validator-addr")

	subCmdDelegate := &cobra.Command{
		Use:   "delegate",
		Short: "delegating to a validator",
		Args:  cobra.ExactArgs(0),
		Long: `
Delegating to a validator
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			networkHandler, err := handlerForShard(0, node)
			if err != nil {
				return err
			}

			_, e := common.NewDecFromString(stakingAmount)
			if e != nil {
				return e
			}

			delegateStakePayloadMaker := func() (staking.Directive, interface{}) {
				amt, _ := common.NewDecFromString(stakingAmount)
				amt = amt.Mul(itcAsDec)

				return staking.DirectiveDelegate, staking.Delegate{
					address.Parse(delegatorAddress.String()),
					address.Parse(validatorAddress.String()),
					amt.RoundInt(),
				}
			}

			nonce, err := getNonce(delegatorAddress.String(), networkHandler)
			if err != nil {
				return err
			}
			stakingTx, err := createStakingTransaction(nonce, delegateStakePayloadMaker)
			if err != nil {
				return err
			}

			passphrase, err = getPassphrase()
			if err != nil {
				return err
			}

			err = handleStakingTransaction(stakingTx, networkHandler, delegatorAddress)
			if err != nil {
				return err
			}
			return nil
		},
	}

	subCmdDelegate.Flags().BoolVar(&trueNonce, "true-nonce", false, "send transaction with on-chain nonce")
	subCmdDelegate.Flags().Var(&delegatorAddress, "delegator-addr", "delegator's address")
	subCmdDelegate.Flags().Var(&validatorAddress, "validator-addr", "validator's address")
	subCmdDelegate.Flags().StringVar(&stakingAmount, "amount", "0", "staking amount")
	subCmdDelegate.Flags().StringVar(&gasPrice, "gas-price", "100", "gas price to pay")
	subCmdDelegate.Flags().StringVar(&gasLimit, "gas-limit", "", "gas limit")
	subCmdDelegate.Flags().StringVar(&inputNonce, "nonce", "", "set nonce for transaction")
	subCmdDelegate.Flags().StringVar(&targetChain, "chain-id", "", "what chain ID to target")
	subCmdDelegate.Flags().Uint32Var(&timeout, "timeout", defaultTimeout, "set timeout in seconds. Set to 0 to not wait for tx confirm")
	subCmdDelegate.Flags().BoolVar(&userProvidesPassphrase, "passphrase", false, ppPrompt)
	subCmdDelegate.Flags().StringVar(&passphraseFilePath, "passphrase-file", "", "path to a file containing the passphrase")

	for _, flagName := range [...]string{"delegator-addr", "validator-addr", "amount"} {
		subCmdDelegate.MarkFlagRequired(flagName)
	}

	subCmdUnDelegate := &cobra.Command{
		Use:   "undelegate",
		Short: "removing delegation responsibility",
		Args:  cobra.ExactArgs(0),
		Long: `
 Removing delegation responsibility
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			networkHandler, err := handlerForShard(0, node)
			if err != nil {
				return err
			}

			_, e := common.NewDecFromString(stakingAmount)
			if e != nil {
				return e
			}

			delegateStakePayloadMaker := func() (staking.Directive, interface{}) {
				amt, _ := common.NewDecFromString(stakingAmount)
				amt = amt.Mul(itcAsDec)

				return staking.DirectiveUndelegate, staking.Undelegate{
					address.Parse(delegatorAddress.String()),
					address.Parse(validatorAddress.String()),
					amt.RoundInt(),
				}
			}

			nonce, err := getNonce(delegatorAddress.String(), networkHandler)
			if err != nil {
				return err
			}
			stakingTx, err := createStakingTransaction(nonce, delegateStakePayloadMaker)
			if err != nil {
				return err
			}

			passphrase, err = getPassphrase()
			if err != nil {
				return err
			}

			err = handleStakingTransaction(stakingTx, networkHandler, delegatorAddress)
			if err != nil {
				return err
			}
			return nil
		},
	}

	subCmdUnDelegate.Flags().BoolVar(&trueNonce, "true-nonce", false, "send transaction with on-chain nonce")
	subCmdUnDelegate.Flags().Var(&delegatorAddress, "delegator-addr", "delegator's address")
	subCmdUnDelegate.Flags().Var(&validatorAddress, "validator-addr", "source validator's address")
	subCmdUnDelegate.Flags().StringVar(&stakingAmount, "amount", "0", "staking amount")
	subCmdUnDelegate.Flags().StringVar(&gasPrice, "gas-price", "100", "gas price to pay")
	subCmdUnDelegate.Flags().StringVar(&gasLimit, "gas-limit", "", "gas limit")
	subCmdUnDelegate.Flags().StringVar(&inputNonce, "nonce", "", "set nonce for transaction")
	subCmdUnDelegate.Flags().StringVar(&targetChain, "chain-id", "", "what chain ID to target")
	subCmdUnDelegate.Flags().Uint32Var(&timeout, "timeout", defaultTimeout, "set timeout in seconds. Set to 0 to not wait for tx confirm")
	subCmdUnDelegate.Flags().BoolVar(&userProvidesPassphrase, "passphrase", false, ppPrompt)
	subCmdUnDelegate.Flags().StringVar(&passphraseFilePath, "passphrase-file", "", "path to a file containing the passphrase")

	for _, flagName := range [...]string{"delegator-addr", "validator-addr", "amount"} {
		subCmdUnDelegate.MarkFlagRequired(flagName)
	}

	subCmdCollectRewards := &cobra.Command{
		Use:   "collect-rewards",
		Short: "collect token rewards",
		Args:  cobra.ExactArgs(0),
		Long: `
Collect token rewards
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			networkHandler, err := handlerForShard(0, node)
			if err != nil {
				return err
			}

			delegateStakePayloadMaker := func() (staking.Directive, interface{}) {
				return staking.DirectiveCollectRewards, staking.CollectRewards{
					address.Parse(delegatorAddress.String()),
				}
			}

			nonce, err := getNonce(delegatorAddress.String(), networkHandler)
			if err != nil {
				return err
			}
			stakingTx, err := createStakingTransaction(nonce, delegateStakePayloadMaker)
			if err != nil {
				return err
			}

			passphrase, err = getPassphrase()
			if err != nil {
				return err
			}

			err = handleStakingTransaction(stakingTx, networkHandler, delegatorAddress)
			if err != nil {
				return err
			}
			return nil
		},
	}

	subCmdCollectRewards.Flags().BoolVar(&trueNonce, "true-nonce", false, "send transaction with on-chain nonce")
	subCmdCollectRewards.Flags().Var(&delegatorAddress, "delegator-addr", "delegator's address")
	subCmdCollectRewards.Flags().StringVar(&gasPrice, "gas-price", "100", "gas price to pay")
	subCmdCollectRewards.Flags().StringVar(&gasLimit, "gas-limit", "", "gas limit")
	subCmdCollectRewards.Flags().StringVar(&inputNonce, "nonce", "", "set nonce for tx")
	subCmdCollectRewards.Flags().StringVar(&targetChain, "chain-id", "", "what chain ID to target")
	subCmdCollectRewards.Flags().Uint32Var(&timeout, "timeout", defaultTimeout, "set timeout in seconds. Set to 0 to not wait for tx confirm")
	subCmdCollectRewards.Flags().BoolVar(&userProvidesPassphrase, "passphrase", false, ppPrompt)
	subCmdCollectRewards.Flags().StringVar(&passphraseFilePath, "passphrase-file", "", "path to a file containing the passphrase")

	for _, flagName := range [...]string{"delegator-addr"} {
		subCmdCollectRewards.MarkFlagRequired(flagName)
	}

	return []*cobra.Command{
		subCmdNewValidator,
		subCmdEditValidator,
		subCmdDelegate,
		subCmdUnDelegate,
		subCmdCollectRewards,
	}
}

func init() {
	cmdStaking := &cobra.Command{
		Use:   "staking",
		Short: "newvalidator, editvalidator, delegate, undelegate or redelegate",
		Long: `
Create a staking transaction, sign it, and send off to the intelchain blockchain
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Help()
			return nil
		},
	}

	cmdStaking.AddCommand(stakingSubCommands()...)
	RootCmd.AddCommand(cmdStaking)
}
