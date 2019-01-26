package main

import (
	"errors"
	"log"
	"os"
	"strconv"

	eth "github.com/ethereum/go-ethereum/common"
	coinbase "github.com/preichenberger/go-gdax"
)

type Coinbase struct {
	client *coinbase.Client
}

func (c *Coinbase) Init() {
	c.client = coinbase.NewClient(
		os.Getenv("CoinbaseSecret"),
		os.Getenv("CoinbaseKey"),
		os.Getenv("CoinbasePassphrase"))
}

func (c *Coinbase) BuyEther() (err error, filledSize float64) {
	log.Printf("Buy £%d worth of ETH on coinbase", EtherValueGBP)

	return nil, 0.1

	// order := coinbase.Order{
	// 	Type:      "market",
	// 	Side:      "buy",
	// 	ProductId: "ETH-EUR",
	// 	Funds:     fmt.Sprintf("%d.00", EtherValueGBP),
	// }

	// result, err := c.client.CreateOrder(&order)
	// if err != nil {
	// 	return errors.New("Failed to buy Ether on Coinbase: " + err.Error()), 0
	// }

	// executedOrder, err := c.client.GetOrder(result.Id)

	// f, err := strconv.ParseFloat(executedOrder.FilledSize, 64)
	// if err != nil {
	// 	return err, 0
	// }

	// return nil, f
}

func (c *Coinbase) SendEther(amount string, to eth.Address) error {

	log.Printf("Send %s ETH from Coinbase to %s", amount, to.Hex())

	var params = CoinbaseWithdrawCryptoParams{
		Amount:        amount,
		Currency:      "ETH",
		CryptoAddress: to.Hex(),
	}
	var result = CoinbaseWithdrawCryptoResult{}

	_, err := c.client.Request(
		"POST",
		"/withdrawals/crypto",
		params,
		&result)

	if err != nil {
		return errors.New("Failed to transfer ETH from Coinbase to user: " + err.Error())
	}

	return nil
}

func (c *Coinbase) GetEtherPrice() (float64, error) {
	b, err := coinbaseClient.client.GetBook("ETH-EUR", 1)
	if err != nil {
		return 0, err
	}
	f, err := strconv.ParseFloat(b.Asks[0].Price, 64)
	if err != nil {
		return 0, err
	}
	return f, nil
}
