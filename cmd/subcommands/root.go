package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/intelchain-itc/itc-sdk/pkg/address"

	color "github.com/fatih/color"
	"github.com/intelchain-itc/itc-sdk/pkg/common"
	"github.com/intelchain-itc/itc-sdk/pkg/rpc"
	rpcEth "github.com/intelchain-itc/itc-sdk/pkg/rpc/eth"
	rpcV1 "github.com/intelchain-itc/itc-sdk/pkg/rpc/v1"
	"github.com/intelchain-itc/itc-sdk/pkg/sharding"
	"github.com/intelchain-itc/itc-sdk/pkg/store"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var (
	verbose         bool
	useLedgerWallet bool
	noLatest        bool
	noPrettyOutput  bool
	node            string
	rpcPrefix       string
	keyStoreDir     string
	givenFilePath   string
	endpoint        = regexp.MustCompile(`https://testnet\.s[0-9]\..*\.intelchain\.network`)
	request         = func(method string, params []interface{}) error {
		if !noLatest {
			params = append(params, "latest")
		}
		success, failure := rpc.Request(method, node, params)
		if failure != nil {
			return failure
		}
		asJSON, _ := json.Marshal(success)
		if noPrettyOutput {
			fmt.Println(string(asJSON))
			return nil
		}
		fmt.Println(common.JSONPrettyFormat(string(asJSON)))
		return nil
	}
	// RootCmd is single entry point of the CLI
	RootCmd = &cobra.Command{
		Use:          "itc",
		Short:        "intelchain blockchain",
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if verbose {
				common.EnableAllVerbose()
			}
			switch rpcPrefix {
			case "itc":
				rpc.Method = rpcV1.Method
			case "eth":
				rpc.Method = rpcEth.Method
			default:
				rpc.Method = rpcV1.Method
			}
			if strings.HasPrefix(node, "https://") || strings.HasPrefix(node, "http://") ||
				strings.HasPrefix(node, "ws://") {
				//No op, already has protocol, respect protocol default ports.
			} else if strings.HasPrefix(node, "api") || strings.HasPrefix(node, "ws") {
				node = "https://" + node
			} else {
				switch URLcomponents := strings.Split(node, ":"); len(URLcomponents) {
				case 1:
					node = "http://" + node + ":9500"
				case 2:
					node = "http://" + node
				default:
					node = node
				}
			}

			if targetChain == "" {
				if node == defaultNodeAddr {
					routes, err := sharding.Structure(node)
					if err != nil {
						chainName = chainIDWrapper{chainID: &common.Chain.TestNet}
					} else {
						if len(routes) == 0 {
							return errors.New("empty reply from sharding structure")
						}
						chainName = endpointToChainID(routes[0].HTTP)
					}
				} else if endpoint.Match([]byte(node)) {
					chainName = endpointToChainID(node)
				} else if strings.Contains(node, "mainnet.s0.intelchain.network") {
					chainName = chainIDWrapper{chainID: &common.Chain.MainNet}
				} else {
					chainName = chainIDWrapper{chainID: &common.Chain.TestNet}
				}
			} else {
				chain, err := common.StringToChainID(targetChain)
				if err != nil {
					return err
				}
				chainName = chainIDWrapper{chainID: chain}
			}

			return nil
		},
		Long: fmt.Sprintf(`
CLI interface to the intelchain blockchain

%s`, g("Invoke 'itc cookbook' for examples of the most common, important usages")),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Help()
			return nil
		},
	}
)

func init() {
	vS := "dump out debug information, same as env var ITC_ALL_DEBUG=true"
	RootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, vS)
	RootCmd.PersistentFlags().StringVarP(&node, "node", "n", defaultNodeAddr, "<host>")
	RootCmd.PersistentFlags().StringVarP(&rpcPrefix, "rpc-prefix", "r", defaultRpcPrefix, "<rpc>")
	RootCmd.PersistentFlags().BoolVar(
		&noLatest, "no-latest", false, "Do not add 'latest' to RPC params",
	)
	RootCmd.PersistentFlags().BoolVar(
		&noPrettyOutput, "no-pretty", false, "Disable pretty print JSON outputs",
	)
	RootCmd.AddCommand(&cobra.Command{
		Use:   "cookbook",
		Short: "Example usages of the most important, frequently used commands",
		RunE: func(cmd *cobra.Command, args []string) error {
			var docNode, docNet string
			if node == defaultNodeAddr || chainName.chainID == &common.Chain.MainNet {
				docNode = `https://testnet.intelchain.network`
				docNet = `Mainnet`
			} else if chainName.chainID == &common.Chain.TestNet {
				docNode = `https://testnet.intelchain.network`
				docNet = `Long-Running Testnet`
			} else if chainName.chainID == &common.Chain.PangaeaNet {
				docNode = `https://testnet.intelchain.network`
				docNet = `Open Staking Network`
			} else if chainName.chainID == &common.Chain.PartnerNet {
				docNode = `https://testnet.intelchain.network`
				docNet = `Partner Testnet`
			} else if chainName.chainID == &common.Chain.StressNet {
				docNode = `https://testnet.intelchain.network`
				docNet = `Stress Testing Network`
			}
			fmt.Print(strings.ReplaceAll(strings.ReplaceAll(cookbookDoc, `[NODE]`, docNode), `[NETWORK]`, docNet))
			return nil
		},
	})
	RootCmd.PersistentFlags().BoolVarP(&useLedgerWallet, "ledger", "e", false, "Use ledger hardware wallet")
	RootCmd.PersistentFlags().StringVar(&givenFilePath, "file", "", "Path to file for given command when applicable")
	RootCmd.AddCommand(&cobra.Command{
		Use:   "docs",
		Short: fmt.Sprintf("Generate docs to a local %s directory", itcDocsDir),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			docDir := path.Join(cwd, itcDocsDir)
			os.Mkdir(docDir, 0700)
			doc.GenMarkdownTree(RootCmd, docDir)
			return nil
		},
	})
}

var (
	// VersionWrapDump meant to be set from main.go
	VersionWrapDump = ""
	cookbook        = color.GreenString("itc cookbook")
	versionLink     = "https://intelchain.org/itccli_ver"
	versionFormat   = regexp.MustCompile("v[0-9]+-[a-z0-9]{7}")
)

// Execute kicks off the itc CLI
func Execute() {
	RootCmd.SilenceErrors = true
	if err := RootCmd.Execute(); err != nil {
		resp, httpErr := http.Get(versionLink)
		if httpErr != nil {
			return
		}
		defer resp.Body.Close()
		// If error, no op
		if resp != nil && resp.StatusCode == 200 {
			buf := new(bytes.Buffer)
			buf.ReadFrom(resp.Body)

			currentVersion := versionFormat.FindAllString(buf.String(), 1)
			if currentVersion != nil && currentVersion[0] != VersionWrapDump {
				warnMsg := fmt.Sprintf("Warning: Using outdated version. Redownload to upgrade to %s\n", currentVersion[0])
				fmt.Fprintf(os.Stderr, color.RedString(warnMsg))
			}
		}
		errMsg := errors.Wrapf(err, "commit: %s, error", VersionWrapDump).Error()
		fmt.Fprintf(os.Stderr, errMsg+"\n")
		fmt.Fprintf(os.Stderr, "check "+cookbook+" for valid examples or try adding a `--help` flag\n")
		os.Exit(1)
	}
}

func endpointToChainID(nodeAddr string) chainIDWrapper {
	if strings.Contains(nodeAddr, ".t.") {
		return chainIDWrapper{chainID: &common.Chain.MainNet}
	} else if strings.Contains(nodeAddr, ".b.") {
		return chainIDWrapper{chainID: &common.Chain.TestNet}
	} else if strings.Contains(nodeAddr, ".os.") {
		return chainIDWrapper{chainID: &common.Chain.PangaeaNet}
	} else if strings.Contains(nodeAddr, ".ps.") {
		return chainIDWrapper{chainID: &common.Chain.PartnerNet}
	} else if strings.Contains(nodeAddr, ".stn.") {
		return chainIDWrapper{chainID: &common.Chain.StressNet}
	} else if strings.Contains(nodeAddr, ".dry.") {
		return chainIDWrapper{chainID: &common.Chain.MainNet}
	}
	return chainIDWrapper{chainID: &common.Chain.TestNet}
}

func validateAddress(cmd *cobra.Command, args []string) error {
	// Check if input valid itc address
	tmpAddr := itcAddress{}
	if err := tmpAddr.Set(args[0]); err != nil {
		// Check if input is valid account name
		if acc, err := store.AddressFromAccountName(args[0]); err == nil {
			addr = itcAddress{acc}
			return nil
		}

		bech32Addr := address.ToBech32(address.Parse(args[0]))
		if bech32Addr == "" {
			return fmt.Errorf("Invalid itc address/Invalid account name: %s", args[0])
		}

		tmpAddr = itcAddress{bech32Addr}
	}
	addr = tmpAddr
	return nil
}
