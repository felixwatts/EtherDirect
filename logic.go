package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"

	eth "github.com/ethereum/go-ethereum/common"
	coinbase "github.com/preichenberger/go-gdax"
	monzo "github.com/tjvr/go-monzo"
)

func PostMonzoFeedError(err error) error {
	return monzoClient.CreateFeedItem(&monzo.FeedItem{
		AccountID: os.Getenv("MonzoAccountId"),
		Title:     "ERROR",
		Body:      err.Error(),
		Type:      "basic",
		ImageURL:  "https://cdn0.iconfinder.com/data/icons/elasto-online-store/26/00-ELASTOFONT-STORE-READY_close-512.png",
	})
}

func PostMonzoFeedInfo(heading string, msg string) error {
	return monzoClient.CreateFeedItem(&monzo.FeedItem{
		AccountID: os.Getenv("MonzoAccountId"),
		Title:     heading,
		Body:      msg,
		Type:      "basic",
		ImageURL:  "https://cdn0.iconfinder.com/data/icons/elasto-online-store/26/00-STORE-37-512.png",
	})
}

func HandleError(err error) {
	if err == nil {
		return
	}

	log.Println(err.Error())

	err = PostMonzoFeedError(err)

	if err != nil {
		log.Println("Failed to post to Monzo feed: " + err.Error())
	}
}

func DepositToMonzoPot(potId string, amountPence uint) error {

	ddid := nextDedupeId
	nextDedupeId = nextDedupeId + 1

	_, err := monzoClient.Deposit(&monzo.DepositRequest{
		PotID:          potId,
		AccountID:      os.Getenv("MonzoAccountId"),
		Amount:         int64(amountPence),
		IdempotencyKey: strconv.FormatInt(ddid, 10),
	})

	if err != nil {
		return errors.New("Failed to deposit to Monzo pot " + potId + ": " + err.Error())
	}

	return nil
}

func IsValidAddress(v string) bool {
	re := regexp.MustCompile("^0x[0-9a-fA-F]{40}$")
	return re.MatchString(v)
}

func ParseOrder(r *http.Request) (err error, tx Order) {
	var data = MonzoWebHook{}

	decoder := json.NewDecoder(r.Body)
	err = decoder.Decode(&data)
	if err != nil {
		return errors.New("Failed to parse request body: " + err.Error()), tx
	}

	if data.Type != "transaction.created" {
		return errors.New("Unexpected WebHook type: " + data.Type), tx
	}

	tx.SortCode = data.Data.CounterParty.SortCode
	tx.AccountNumber = data.Data.CounterParty.AccountNumber
	tx.Amount = data.Data.Amount
	tx.Currency = data.Data.Currency

	if tx.SortCode == "" || tx.AccountNumber == "" {
		return errors.New("Counterparty data missing"), tx
	}

	if data.Data.Amount != OrderAmountPence {
		return errors.New(fmt.Sprintf("Wrong amount. Send £%.2f exactly", OrderAmountPence/100.0)), tx
	}

	if data.Data.Currency != "GBP" {
		return errors.New("Wrong currency. Send GBP only"), tx
	}

	if !IsValidAddress(data.Data.Description) {
		return errors.New("Reference field of transfer must contain a valid Ethereum address"), tx
	}

	tx.EthAddress = eth.HexToAddress(data.Data.Description)

	return nil, tx
}

func Refund(tx Order, err error) error {

	if tx.SortCode == "" || tx.AccountNumber == "" || tx.Currency == "" {
		return errors.New("An error occurred but we do not have enough information to issue a refund: " + err.Error())
	}

	PostMonzoFeedInfo("REFUND", fmt.Sprintf("%s %s %d %s %s", tx.SortCode, tx.AccountNumber, tx.Amount, tx.Currency, err.Error()))

	err2 := DepositToMonzoPot(os.Getenv("MonzoPotIdRefund"), tx.Amount)

	if err2 != nil {
		return errors.New("Failed to deposit into Refund pot: " + err2.Error() + ". Original error: " + err.Error())
	}

	return err
}

func BuyEtherOnCoinbase() (err error, filledSize string) {
	log.Printf("Buy £%d worth of ETH on coinbase", EtherValueGBP)

	order := coinbase.Order{
		Type:      "market",
		Side:      "buy",
		ProductId: "ETH-EUR",
		Funds:     fmt.Sprintf("%d.00", EtherValueGBP),
	}

	result, err := coinbaseClient.CreateOrder(&order)
	if err != nil {
		return errors.New("Failed to buy Ether on Coinbase: " + err.Error()), "0"
	}

	executedOrder, err := coinbaseClient.GetOrder(result.Id)

	return nil, executedOrder.FilledSize
}

func SendEtherFromCoinbaseToUser(amount string, to eth.Address) error {

	log.Printf("Send %s ETH from Coinbase to %s", amount, to.Hex())

	var params = CoinbaseWithdrawCryptoParams{
		Amount:        amount,
		Currency:      "ETH",
		CryptoAddress: to.Hex(),
	}
	var result = CoinbaseWithdrawCryptoResult{}

	_, err := coinbaseClient.Request(
		"POST",
		"/withdrawals/crypto",
		params,
		&result)

	if err != nil {
		return errors.New("Failed to transfer ETH from Coinbase to user: " + err.Error())
	}

	return nil
}

func SendGbpFromMonzoToCoinbase() error {
	return DepositToMonzoPot(os.Getenv("MonzoPotIdCoinbase"), EtherValueGBP*100)
}

func ProcessOrder(w http.ResponseWriter, r *http.Request) error {

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return errors.New("Invalid request method: " + r.Method)
	}

	// TODO verify that this is really from Monzo

	// Parse and validate the incoming bank transfer
	err, order := ParseOrder(r)
	log.Println(order)
	if err != nil {
		// If it's invalid refund the user
		return Refund(order, err)
	}

	// Try to buy ether
	err, amount := BuyEtherOnCoinbase()
	if err != nil {
		// If buying ether fails, refund the user
		return Refund(order, err)
	}

	// Try to send Ether to user
	err = SendEtherFromCoinbaseToUser(amount, order.EthAddress)
	if err != nil {
		// If sending ether to user fails, refund
		return Refund(order, err)
		// TODO try to sell the ether on coinbase?
	}

	// Interaction with user is complete, now try to balance our internal books

	err = SendGbpFromMonzoToCoinbase()
	if err != nil {
		return err
	}

	err = DepositToMonzoPot(os.Getenv("MonzoPotIdProfit"), ServiceChargeGBP*100)

	return err
}
