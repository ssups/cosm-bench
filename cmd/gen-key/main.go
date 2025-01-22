package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"golang.org/x/sync/semaphore"
)

const (
	keyringAppName   = "cronos"
	keyringBackend   = keyring.BackendTest
	keyFilePrefix    = "test-key-" // key file will be generated => {keyFilePrefix}0.info, {keyFilePrefix}1.info ...
	concurrencyLimit = 100
	homePath         = "node/node1"
)

var (
	// eg) go run gen-key/main.go --a 200
	numAccounts = flag.Int("a", 100, "number of accounts to generate")
)

func makeCodec() codec.Codec {
	interfaceRegistry := types.NewInterfaceRegistry()
	marshaler := codec.NewProtoCodec(interfaceRegistry)
	cryptocodec.RegisterInterfaces(interfaceRegistry)
	return marshaler
}

func createKey(index int, kr keyring.Keyring) {
	keyName := fmt.Sprintf("%s%d", keyFilePrefix, index)
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

	err := os.MkdirAll(homePath, 0755)
	if err != nil {
		fmt.Printf("Failed to create home directory: %v\n", err)
		return
	}

	cdc := makeCodec()

	kr, err := keyring.New(
		keyringAppName,
		keyringBackend,
		homePath,
		os.Stdin,
		cdc,
	)
	if err != nil {
		fmt.Printf("Failed to initialize keyring: %v\n", err)
		return
	}

	sem := semaphore.NewWeighted(int64(concurrencyLimit))

	fmt.Printf("Starting to generate %d keys...\n", *numAccounts)

	for i := 0; i < *numAccounts; i++ {
		if err := sem.Acquire(context.Background(), 1); err != nil {
			log.Printf("Failed to acquire semaphore: %v", err)
			break
		}

		go func() {
			i := i
			defer sem.Release(1)
			createKey(i, kr)
		}()

		if (i+1)%1000 == 0 {
			fmt.Printf("Progress: %d/%d keys initiated...\n", i+1, *numAccounts)
		}
	}

	if err := sem.Acquire(context.Background(), int64(concurrencyLimit)); err != nil {
		log.Printf("Failed to acquire semaphore: %v", err)
	}

	fmt.Printf("\nCompleted generating %d keys\n", *numAccounts)
	fmt.Printf("\nYou can check keys using:\ncronosd keys list --keyring-backend test --home %s\n", homePath)
}
