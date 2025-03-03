package cmd

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	bls_core "github.com/intelchain-itc/bls/ffi/go/bls"
	"github.com/intelchain-itc/intelchain/crypto/bls"
	"github.com/intelchain-itc/itc-sdk/pkg/address"
	"github.com/intelchain-itc/itc-sdk/pkg/rpc"
	"github.com/spf13/cobra"
)

func init() {
	cmdUtilities := &cobra.Command{
		Use:   "utility",
		Short: "common intelchain blockchain utilities",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Help()
			return nil
		},
	}

	cmdUtilities.AddCommand(&cobra.Command{
		Use:   "metadata",
		Short: "data includes network specific values",
		RunE: func(cmd *cobra.Command, args []string) error {
			noLatest = true
			return request(rpc.Method.GetNodeMetadata, []interface{}{})
		},
	})

	cmdMetrics := &cobra.Command{
		Use:   "metrics",
		Short: "mostly in-memory fluctuating values",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Help()
			return nil
		},
	}

	cmdMetrics.AddCommand([]*cobra.Command{{
		Use:   "pending-crosslinks",
		Short: "dump the pending crosslinks in memory of target node",
		RunE: func(cmd *cobra.Command, args []string) error {
			noLatest = true
			return request(rpc.Method.GetPendingCrosslinks, []interface{}{})
		},
	}, {
		Use:   "pending-cx-receipts",
		Short: "dump the pending cross shard receipts in memory of target node",
		RunE: func(cmd *cobra.Command, args []string) error {
			noLatest = true
			return request(rpc.Method.GetPendingCXReceipts, []interface{}{})
		},
	},
	}...)

	cmdUtilities.AddCommand(cmdMetrics)
	cmdUtilities.AddCommand([]*cobra.Command{{
		Use:   "bech32-to-addr",
		Args:  cobra.ExactArgs(1),
		Short: "0x Address of a bech32 itc-address",
		RunE: func(cmd *cobra.Command, args []string) error {
			addr, err := address.Bech32ToAddress(args[0])
			if err != nil {
				return err
			}
			fmt.Println(addr.Hex())
			return nil
		},
	}, {
		Use:   "addr-to-bech32",
		Args:  cobra.ExactArgs(1),
		Short: "bech32 itc-address of an 0x address",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(address.ToBech32(address.Parse(args[0])))
			return nil
		},
	}, {
		Use:   "committees",
		Short: "current and previous committees",
		RunE: func(cmd *cobra.Command, args []string) error {
			noLatest = true
			return request(rpc.Method.GetSuperCommmittees, []interface{}{})
		},
	}, {
		Use:   "bad-blocks",
		Short: "bad blocks in memory of the target node",
		RunE: func(cmd *cobra.Command, args []string) error {
			noLatest = true
			return request(rpc.Method.GetCurrentBadBlocks, []interface{}{})
		},
	}, {
		Use:   "shards",
		Short: "sharding structure and end points",
		RunE: func(cmd *cobra.Command, args []string) error {
			noLatest = true
			return request(rpc.Method.GetShardingStructure, []interface{}{})
		},
	}, {
		// Temp utility that should be removed once resharding implemented
		Use:   "shard-for-bls",
		Args:  cobra.ExactArgs(1),
		Short: "which shard this BLS key would be assigned to",
		RunE: func(cmd *cobra.Command, args []string) error {
			inputKey := strings.TrimPrefix(args[0], "0x")
			key := bls_core.PublicKey{}
			if err := key.DeserializeHexStr(inputKey); err != nil {
				return err
			}
			reply, err := rpc.Request(rpc.Method.GetShardingStructure, node, []interface{}{})
			if err != nil {
				return err
			}
			shardBig := len(reply["result"].([]interface{})) // assume the response is a JSON Array
			wrapper := bls.FromLibBLSPublicKeyUnsafe(&key)
			shardID := int(new(big.Int).Mod(wrapper.Big(), big.NewInt(int64(shardBig))).Int64())
			type t struct {
				ShardID int `json:"shard-id"`
			}
			result, err := json.Marshal(t{shardID})
			if err != nil {
				return err
			}

			fmt.Println(string(result))
			return nil
		},
	}, {
		Use:   "last-cross-links",
		Short: "last crosslinks processed",
		RunE: func(cmd *cobra.Command, args []string) error {
			noLatest = true
			return request(rpc.Method.GetLastCrossLinks, []interface{}{})
		},
	}}...)

	RootCmd.AddCommand(cmdUtilities)
}
