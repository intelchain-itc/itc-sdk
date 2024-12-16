package cmd

import (
	"github.com/intelchain-itc/go-sdk/pkg/address"
	"github.com/intelchain-itc/go-sdk/pkg/common"
	"github.com/intelchain-itc/go-sdk/pkg/validation"
	"github.com/pkg/errors"
)

type itcAddress struct {
	address string
}

func (itcAddress itcAddress) String() string {
	return itcAddress.address
}

func (itcAddress *itcAddress) Set(s string) error {
	err := validation.ValidateAddress(s)
	if err != nil {
		return err
	}
	_, err = address.Bech32ToAddress(s)
	if err != nil {
		return errors.Wrap(err, "not a valid itc address")
	}
	itcAddress.address = s
	return nil
}

func (itcAddress itcAddress) Type() string {
	return "itc-address"
}

type chainIDWrapper struct {
	chainID *common.ChainID
}

func (chainIDWrapper chainIDWrapper) String() string {
	return chainIDWrapper.chainID.Name
}

func (chainIDWrapper *chainIDWrapper) Set(s string) error {
	chain, err := common.StringToChainID(s)
	chainIDWrapper.chainID = chain
	if err != nil {
		return err
	}
	return nil
}

func (chainIDWrapper chainIDWrapper) Type() string {
	return "chain-id"
}
