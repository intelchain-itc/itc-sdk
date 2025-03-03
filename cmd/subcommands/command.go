package cmd

import (
	"log"
	"os"
	"path"

	ethereum_rpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/intelchain-itc/itc-sdk/pkg/common"
	"github.com/intelchain-itc/itc-sdk/pkg/console"
	"github.com/intelchain-itc/itc-sdk/pkg/rpc"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
)

func init() {
	net := "mainnet"

	cmdCommand := &cobra.Command{
		Use:   "command",
		Short: "Start an interactive JavaScript environment (connect to node)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return openConsole(net)
		},
	}

	cmdCommand.Flags().StringVar(&net, "net", "mainnet", "used net(default: mainnet, eg: mainnet, testnet ...)")

	RootCmd.AddCommand(cmdCommand)
}

func checkAndMakeDirIfNeeded() string {
	userDir, _ := homedir.Dir()
	itcCLIDir := path.Join(userDir, common.DefaultConfigDirName, common.DefaultCommandAliasesDirName)
	if _, err := os.Stat(itcCLIDir); os.IsNotExist(err) {
		// Double check with Leo what is right file persmission
		os.Mkdir(itcCLIDir, 0700)
	}

	return itcCLIDir
}

// remoteConsole will connect to a remote node instance, attaching a JavaScript
// console to it.
func openConsole(net string) error {
	client, err := ethereum_rpc.Dial(node)
	if err != nil {
		log.Fatalf("Unable to attach to remote node: %v", err)
	}

	// check net type
	_, err = common.StringToChainID(net)
	if err != nil {
		return err
	}

	// get shard id
	nodeRPCReply, err := rpc.Request(rpc.Method.GetShardID, node, []interface{}{})
	if err != nil {
		return err
	}

	shard := int(nodeRPCReply["result"].(float64))

	config := console.Config{
		DataDir: checkAndMakeDirIfNeeded(),
		DocRoot: ".",
		Client:  client,
		Preload: nil,
		NodeUrl: node,
		ShardId: shard,
		Net:     net,
	}

	consoleInstance, err := console.New(config)
	if err != nil {
		log.Fatalf("Failed to start the JavaScript console insatnce: %v", err)
	}
	defer consoleInstance.Stop(false)

	// Otherwise print the welcome screen and enter interactive mode
	consoleInstance.Welcome()
	consoleInstance.Interactive()

	return nil
}
