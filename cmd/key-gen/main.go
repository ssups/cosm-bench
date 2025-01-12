package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	keyringPassphrase = "test"
	keyringAppName    = "cronos"
	keyringBackend    = keyring.BackendTest
	concurrencyLimit  = 100
	homeDir           = "node/node1"
)

var (
	numAccounts = flag.Int("a", 2, "number of accounts to generate")
)

func MakeCodec() codec.Codec {
	interfaceRegistry := types.NewInterfaceRegistry()
	marshaler := codec.NewProtoCodec(interfaceRegistry)
	cryptocodec.RegisterInterfaces(interfaceRegistry)
	return marshaler
}

func init() {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount("crc", "crcpub")
	config.SetBech32PrefixForValidator("crcvaloper", "crcvaloperpub")
	config.SetBech32PrefixForConsensusNode("crcvalcons", "crcvalconspub")
	config.Seal()
}

func createKey(index int, kr keyring.Keyring, wg *sync.WaitGroup) {
	defer wg.Done()

	keyName := fmt.Sprintf("test-key-%d", index)
	_, err := kr.Key(keyName)
	if err == nil {
		fmt.Printf("Key %s already exists, skipping...\n", keyName)
		return
	}

	privKey := secp256k1.GenPrivKey()
	err = kr.ImportPrivKeyHex(keyName, hex.EncodeToString(privKey.Bytes()), "secp256k1")
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	flag.Parse()

	err := os.MkdirAll(homeDir, 0755)
	if err != nil {
		fmt.Printf("Failed to create home directory: %v\n", err)
		return
	}

	cdc := MakeCodec()

	kr, err := keyring.New(
		keyringAppName,
		keyringBackend,
		homeDir,
		os.Stdin,
		cdc,
	)
	if err != nil {
		fmt.Printf("Failed to initialize keyring: %v\n", err)
		return
	}

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, concurrencyLimit)

	fmt.Printf("Starting to generate %d keys...\n", *numAccounts)

	for i := 1; i <= *numAccounts; i++ {
		wg.Add(1)
		semaphore <- struct{}{}

		go func(index int) {
			defer func() { <-semaphore }()
			createKey(index, kr, &wg)
		}(i)

		if (i+1)%1000 == 0 {
			fmt.Printf("Progress: %d/%d keys initiated...\n", i+1, *numAccounts)
		}
	}

	wg.Wait()
	fmt.Printf("\nCompleted generating %d keys\n", *numAccounts)
	fmt.Printf("\nYou can check keys using:\ncronosd keys list --keyring-backend test --home %s\n", homeDir)
}
