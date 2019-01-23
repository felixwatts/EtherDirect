package main

import (
	"fmt"

	eth "github.com/ethereum/go-ethereum/common"
)

type IndexViewModel struct {
}

type MonzoWebHookCounterParty struct {
	Name          string
	SortCode      string `json:"sort_code"`
	AccountNumber string `json:"account_number"`
}

type MonzoWebHookTransaction struct {
	Description  string
	Amount       uint
	Currency     string
	CounterParty MonzoWebHookCounterParty
}

type MonzoWebHook struct {
	Type string
	Data MonzoWebHookTransaction
}

type Order struct {
	SortCode      string
	AccountNumber string
	Currency      string
	Amount        uint
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
