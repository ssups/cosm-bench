package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	"golang.org/x/sync/errgroup"
)

const (
	keyringAppName   = "cronos"
	keyringBackend   = keyring.BackendTest
	homePath         = "node/node1"
	tokenDenom       = "stake"
	concurrencyLimit = 100
)

var (
	numAccounts = flag.Int("a", 2, "number of accounts to generate")
)

func makeCodec() (*codec.ProtoCodec, *types.InterfaceRegistry) {
	interfaceRegistry := types.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(interfaceRegistry)
	cryptocodec.RegisterInterfaces(interfaceRegistry)
	authtypes.RegisterInterfaces(interfaceRegistry)
	return cdc, &interfaceRegistry
}

func init() {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount("crc", "crcpub")
	// config.SetBech32PrefixForValidator("crcvaloper", "crcvaloperpub")
	// config.SetBech32PrefixForConsensusNode("crcvalcons", "crcvalconspub")
	// config.Seal()
}

func main() {
	flag.Parse()
	cdc, _ := makeCodec()

	kr, err := keyring.New(
		keyringAppName,
		keyringBackend,
		homePath,
		os.Stdin,
		cdc,
	)
	if err != nil {
		log.Fatalf("Failed to initialize keyring: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}
	genesisFile := filepath.Join(wd, homePath, "config", "genesis.json")

	appState, genDoc, err := genutiltypes.GenesisStateFromGenFile(genesisFile)
	if err != nil {
		log.Fatalf("Failed to read genesis state: %v", err)
	}

	authGenState := authtypes.GetGenesisStateFromAppState(cdc, appState)
	bankGenState := banktypes.GetGenesisStateFromAppState(cdc, appState)

	preAccountsLen := len(authGenState.Accounts)
	amount, ok := math.NewIntFromString("1000000000000000000000")
	if !ok {
		log.Fatal("failed to parse amount")
	}

	coins := sdk.NewCoins(
		sdk.NewCoin(tokenDenom, amount),
	)

	additionAccounts := make([]*types.Any, *numAccounts)
	additionBalances := make([]banktypes.Balance, *numAccounts)

	g := new(errgroup.Group)
	g.SetLimit(concurrencyLimit)

	for i := 0; i < *numAccounts; i++ {
		i := i
		g.Go(func() error {
			keyName := fmt.Sprintf("test-key-%d", i)
			key, err := kr.Key(keyName)
			if err != nil {
				log.Fatal(err)
			}

			addr, err := key.GetAddress()
			if err != nil {
				log.Fatal(err)
			}

			pub, err := key.GetPubKey()
			if err != nil {
				log.Fatal(err)
			}

			acc := authtypes.NewBaseAccount(addr, pub, uint64(preAccountsLen+i), 0)
			if err := acc.Validate(); err != nil {
				log.Fatalf("Failed to validate account: %v", err)
			}

			anyacc, err := types.NewAnyWithValue(acc)
			if err != nil {
				log.Fatal(err)
			}

			additionAccounts[i] = anyacc
			additionBalances[i] = banktypes.Balance{
				Address: addr.String(),
				Coins:   coins,
			}
			// authGenState.Accounts = append(authGenState.Accounts, anyacc)
			// bankGenState.Balances = append(bankGenState.Balances, banktypes.Balance{
			// 	Address: addr.String(),
			// 	Coins:   coins,
			// })

			return nil
		})
	}

	g.Wait()

	authGenState.Accounts = append(authGenState.Accounts, additionAccounts...)
	bankGenState.Balances = append(bankGenState.Balances, additionBalances...)
	totalAddedAmount := amount.Mul(math.NewInt(int64(*numAccounts)))
	bankGenState.Supply = bankGenState.Supply.Add(sdk.NewCoin(tokenDenom, totalAddedAmount))

	appState[authtypes.ModuleName] = cdc.MustMarshalJSON(&authGenState)
	appState[banktypes.ModuleName] = cdc.MustMarshalJSON(bankGenState)
	appStateJSON, err := json.Marshal(appState)
	if err != nil {
		log.Fatalf("Failed to marshal appState: %v", err)
	}

	genDoc.AppState = appStateJSON
	err = genutil.ExportGenesisFile(genDoc, genesisFile)
	if err != nil {
		log.Fatalf("Failed to export genesis file: %v", err)
	}

	fmt.Printf("Successfully added %d accounts to genesis\n", *numAccounts)
}
