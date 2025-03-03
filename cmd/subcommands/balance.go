package cmd

import (
	"bytes"
	"fmt"
	"net"
	"strings"

	"github.com/intelchain-itc/itc-sdk/pkg/common"
	"github.com/intelchain-itc/itc-sdk/pkg/rpc"
	"github.com/intelchain-itc/itc-sdk/pkg/sharding"
	"github.com/spf13/cobra"
)

func init() {
	cmdQuery := &cobra.Command{
		Use:     "balances",
		Short:   "Check account balance on all shards",
		Long:    "Query for the latest account balance given a Intelchain Address",
		Args:    cobra.ExactArgs(1),
		PreRunE: validateAddress,
		RunE: func(cmd *cobra.Command, args []string) error {
			if checkNodeInput(node) {
				balanceRPCReply, err := rpc.Request(rpc.Method.GetBalance, node, []interface{}{addr.address, "latest"})
				if err != nil {
					return err
				}
				nodeRPCReply, err := rpc.Request(rpc.Method.GetShardID, node, []interface{}{})
				if err != nil {
					return err
				}
				balance, _ := balanceRPCReply["result"].(string)
				bln := common.NewDecFromHex(balance)
				bln = bln.Quo(itcAsDec)
				var out bytes.Buffer
				out.WriteString("[")
				out.WriteString(fmt.Sprintf(`{"shard":%d, "amount":%s}`,
					uint32(nodeRPCReply["result"].(float64)),
					bln.String(),
				))
				out.WriteString("]")
				fmt.Println(common.JSONPrettyFormat(out.String()))
				return nil
			}
			r, err := sharding.CheckAllShards(node, addr.String(), noPrettyOutput)
			if err != nil {
				return err
			}
			fmt.Println(r)
			return nil
		},
	}

	RootCmd.AddCommand(cmdQuery)
}

// Check if input for --node is an IP address
func checkNodeInput(node string) bool {
	removePrefix := strings.TrimPrefix(node, "http://")
	removePrefix = strings.TrimPrefix(removePrefix, "https://")
	possibleIP := strings.Split(removePrefix, ":")[0]
	return net.ParseIP(string(possibleIP)) != nil
}
