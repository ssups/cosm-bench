package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"os"

	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"

	// authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	chainID          = "cronos_777-1"
	keyringAppName   = "cronos"
	keyringBackend   = keyring.BackendTest
	denom            = "stake"
	homePath         = "node/node1"
	concurrencyLimit = 100
	grpcEndpoint     = "localhost:9090"
)

var (
	numAccounts = flag.Int("a", 2, "Number of accounts to make tx")
)

type txCreateConfig struct {
	client.TxConfig
	codec.Codec
	types.InterfaceRegistry
}

func newTxConfig() *txCreateConfig {
	registry := types.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)
	cryptocodec.RegisterInterfaces(registry)
	banktypes.RegisterInterfaces(registry)
	authtypes.RegisterInterfaces(registry)

	txConfig := authtx.NewTxConfig(cdc, []signing.SignMode{signing.SignMode_SIGN_MODE_DIRECT})

	return &txCreateConfig{
		TxConfig:          txConfig,
		Codec:             cdc,
		InterfaceRegistry: registry,
	}
}

type txCreator struct {
	txConfig   *txCreateConfig
	kr         keyring.Keyring
	authClient authtypes.QueryClient
	factory    tx.Factory
	coins      sdk.Coins
}

func newTxCreator() (*txCreator, error) {
	txConfig := newTxConfig()

	kr, err := keyring.New(
		keyringAppName,
		keyringBackend,
		homePath,
		os.Stdin,
		txConfig.Codec,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize keyring: %v", err)
	}

	grpcConn, err := grpc.NewClient(grpcEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gRPC: %v", err)
	}

	factory := tx.Factory{}.
		WithTxConfig(txConfig.TxConfig).
		WithKeybase(kr).
		WithChainID(chainID).
		WithSignMode(signing.SignMode_SIGN_MODE_DIRECT)

	return &txCreator{
		txConfig:   txConfig,
		kr:         kr,
		authClient: authtypes.NewQueryClient(grpcConn),
		factory:    factory,
		coins:      sdk.NewCoins(sdk.NewCoin(denom, math.NewInt(100))),
	}, nil
}

func (tc *txCreator) createSignedTx(ctx context.Context, accountIndex int) ([]byte, error) {
	keyName := fmt.Sprintf("test-key-%d", accountIndex)

	key, err := tc.kr.Key(keyName)
	if err != nil {
		return nil, fmt.Errorf("failed to get key %s: %v", keyName, err)
	}

	addr, err := key.GetAddress()
	if err != nil {
		return nil, fmt.Errorf("failed to get address %s: %v", addr, err)
	}

	fromAddr := sdk.AccAddress(addr)
	res, err := tc.authClient.Account(ctx, &authtypes.QueryAccountRequest{
		Address: fromAddr.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query account %s: %v", fromAddr, err)
	}

	var account sdk.AccountI
	if err := tc.txConfig.InterfaceRegistry.UnpackAny(res.Account, &account); err != nil {
		return nil, fmt.Errorf("failed to unpack account: %v", err)
	}

	txBuilder := tc.txConfig.TxConfig.NewTxBuilder()
	msg := banktypes.NewMsgSend(fromAddr, fromAddr, tc.coins)
	if err := txBuilder.SetMsgs(msg); err != nil {
		return nil, fmt.Errorf("failed to set msgs: %v", err)
	}

	txFactory := tc.factory.WithAccountNumber(account.GetAccountNumber()).WithSequence(account.GetSequence())
	txBuilder.SetGasLimit(200000)
	// txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin(denom, math.NewInt(2000))))
	if err := tx.Sign(ctx, txFactory, keyName, txBuilder, false); err != nil {
		return nil, fmt.Errorf("failed to sign: %v", err)
	}

	return tc.txConfig.TxConfig.TxEncoder()(txBuilder.GetTx())
}

func init() {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount("crc", "crcpub")
	config.Seal()
}

func main() {
	flag.Parse()

	creator, err := newTxCreator()
	if err != nil {
		log.Fatalf("failed to setup tx creator: %v", err)
	}

	if err := os.MkdirAll("node/txns/encoded", 0755); err != nil {
		log.Fatalf("failed to create directory: %v", err)
	}

	ctx := context.TODO()
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(concurrencyLimit)

	for i := 0; i < *numAccounts; i++ {
		i := i

		g.Go(func() error {
			txBytes, err := creator.createSignedTx(ctx, i)
			if err != nil {
				return fmt.Errorf("failed to create tx for account %d: %w", i, err)
			}

			err = os.WriteFile(
				fmt.Sprintf("node/txns/encoded/txns%d.json", i),
				bytes.NewBufferString(base64.StdEncoding.EncodeToString(txBytes)).Bytes(),
				0644,
			)
			if err != nil {
				return fmt.Errorf("failed to write file for account %d: %w", i, err)
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		log.Fatalf("error during execution: %v", err)
	}

	fmt.Printf("Successfully generated %d encode tranactions\n", *numAccounts)
}
