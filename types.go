package main

import (
	"fmt"

	eth "github.com/ethereum/go-ethereum/common"
)

type IndexViewModel struct {
}

type Order struct {
	SortCode      string
	AccountNumber string
	Currency      string
	Amount        int
	EthAddress    eth.Address
}

func (o Order) String() string {
	return fmt.Sprintf("{ %s %s %s %d %s }", o.SortCode, o.AccountNumber, o.Currency, o.Amount, o.EthAddress.Hex())
}

type CoinbaseWithdrawCryptoParams struct {
	Amount        string `json:"amount"`
	Currency      string `json:"currency"`
	CryptoAddress string `json:"crypto_address"`
}

type CoinbaseWithdrawCryptoResult struct {
	Id       string `json:"id"`
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}
