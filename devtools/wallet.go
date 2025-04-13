package main

import (
	"context"
	"log"
	"strings"

	"github.com/xssnick/tonutils-go/liteclient"
	"github.com/xssnick/tonutils-go/ton"
	"github.com/xssnick/tonutils-go/ton/wallet"
)

func main() {
	ctx := context.Background()

	// Connect to lite server
	client := liteclient.NewConnectionPool()
	cfg, err := liteclient.GetConfigFromUrl(ctx, "https://ton.org/testnet-global.config.json")
	if err != nil {
		log.Fatalln("get config err: ", err.Error())
		return
	}

	err = client.AddConnectionsFromConfig(ctx, cfg)
	if err != nil {
		log.Fatalln("connection err: ", err.Error())
		return
	}

	// api client with full proof checks
	api := ton.NewAPIClient(client, ton.ProofCheckPolicyFast).WithRetry()
	api.SetTrustedBlockFromConfig(cfg)

	// mnemonic := "crane flavor south loyal volcano tribe shock nerve space account broken movie stomach salt catch script blush excuse draw lava square oyster tragic display"

	// wallet address: UQDIEc1Xfx5drYSUAO3vUoJucXaqthaSDpzxBtR6lV1fVyia
	// testnet address: 0QDIEc1Xfx5drYSUAO3vUoJucXaqthaSDpzxBtR6lV1fV5MQ

	seed := wallet.NewSeed()
	log.Println("New seed:", strings.Join(seed, " "))

	// initialize high-load wallet
	w, err := wallet.FromSeed(api, seed, wallet.ConfigHighloadV3{
		MessageTTL: 60 * 5,
		MessageBuilder: func(ctx context.Context, subWalletId uint32) (id uint32, createdAt int64, err error) {
			return 0, 0, nil
		},
	})

	if err != nil {
		log.Fatalln("FromSeed err:", err.Error())
		return
	}

	log.Println("testnet wallet address:", w.WalletAddress().Testnet(true))

	block, err := api.CurrentMasterchainInfo(context.Background())
	if err != nil {
		log.Fatalln("CurrentMasterchainInfo err:", err.Error())
		return
	}

	log.Println("block:", block)

	balance, err := w.GetBalance(context.Background(), block)
	if err != nil {
		log.Fatalln("GetBalance err:", err.Error())
		return
	}

	log.Println("Balance:", balance)

	if balance.Nano().Uint64() >= 3000000 {
		log.Println("Have balance")
	}

	log.Println("not enough balance:", balance.String())
	// Create HighloadWalletV3
	/*w, err := wallet.NewHighloadV3Wallet(client, key.Public(), 0, true)
	if err != nil {
		log.Fatalf("wallet creation error: %v", err)
	}

	// Print future wallet address
	fmt.Println("New HighloadWalletV3 address:", w.Address().String())

	// Fund the wallet with some TON before deploying (min ~0.05-0.1 TON)
	// You can use https://tonkeeper.com/ or testnet faucet if using testnet

	// Deploy the wallet to chain
	err = w.Deploy(ctx, key.Private())
	if err != nil {
		log.Fatalf("Deploy error: %v", err)
	}

	fmt.Println("Wallet deployed successfully")*/
}
