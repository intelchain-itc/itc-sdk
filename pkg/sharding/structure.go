package sharding

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/intelchain-itc/intelchain/common/denominations"
	"github.com/intelchain-itc/intelchain/numeric"
	"github.com/intelchain-itc/itc-sdk/pkg/common"
	"github.com/intelchain-itc/itc-sdk/pkg/rpc"
)

var (
	ticksAsDec = numeric.NewDec(denominations.Ticks)
	itcAsDec   = numeric.NewDec(denominations.Itc)

	localToPublicEndpoints = map[string]string{
		"http://127.0.0.1:9500": "https://testnet.intelchain.network",
		"http://127.0.0.1:9502": "https://testnet.s1.intelchain.network",
		"ws://127.0.0.1:9800":   "wss://testnet.s0.intelchain.network/ws",
		"ws://127.0.0.1:9802":   "wss://testnet.s1.intelchain.network/ws",
	}
)

func rewriteEndpoint(endpoint string) string {
	if newEndpoint, exists := localToPublicEndpoints[endpoint]; exists {
		return newEndpoint
	}
	return endpoint
}

// RPCRoutes reflects the RPC endpoints of the target network across shards
type RPCRoutes struct {
	HTTP    string `json:"http"`
	ShardID int    `json:"shardID"`
	WS      string `json:"ws"`
}

// Structure produces a slice of RPCRoutes for the network across shards
func Structure(node string) ([]RPCRoutes, error) {
	type r struct {
		Result []RPCRoutes `json:"result"`
	}
	p, e := rpc.RawRequest(rpc.Method.GetShardingStructure, node, []interface{}{})
	if e != nil {
		return nil, e
	}
	result := r{}
	if err := json.Unmarshal(p, &result); err != nil {
		return nil, err
	}

	// Add this new section to rewrite the endpoints
	for i := range result.Result {
		result.Result[i].HTTP = rewriteEndpoint(result.Result[i].HTTP)
		result.Result[i].WS = rewriteEndpoint(result.Result[i].WS)
	}

	return result.Result, nil
}

func CheckAllShards(node, itcAddr string, noPretty bool) (string, error) {
	var out bytes.Buffer
	out.WriteString("[")
	params := []interface{}{itcAddr, "latest"}
	s, err := Structure(node)
	if err != nil {
		return "", err
	}
	for i, shard := range s {
		balanceRPCReply, err := rpc.Request(rpc.Method.GetBalance, shard.HTTP, params)
		if err != nil {
			if common.DebugRPC {
				fmt.Printf("NOTE: Route %s failed.", shard.HTTP)
			}
			continue
		}
		if i != 0 {
			out.WriteString(",")
		}
		balance, _ := balanceRPCReply["result"].(string)
		bln := common.NewDecFromHex(balance)
		bln = bln.Quo(itcAsDec)
		out.WriteString(fmt.Sprintf(`{"shard":%d, "amount":%s}`,
			shard.ShardID,
			bln.String(),
		))
	}
	out.WriteString("]")
	if noPretty {
		return out.String(), nil
	}
	return common.JSONPrettyFormat(out.String()), nil
}
