package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"

	eth "github.com/ethereum/go-ethereum/common"
)

func HandleError(err error) {
	if err == nil {
		return
	}

	log.Println(err.Error())

	err = monzoClient.PostError(err)

	if err != nil {
		log.Println("Failed to post to Monzo feed: " + err.Error())
	}
}

func IsValidAddress(v string) bool {
	re := regexp.MustCompile("^0x[0-9a-fA-F]{40}$")
	return re.MatchString(v)
}

func AccessCodeToEthereumAddress(accessCode string) (string, error) {
	dat, err := ioutil.ReadFile(fmt.Sprintf("%saccess-codes/%s.txt", FileSystemRoot, accessCode))
	if err != nil {
		return "", err
	}
	return string(dat), nil
}

func ParseOrder(r *http.Request) (err error, tx Order) {
	var data = MonzoWebHook{}

	decoder := json.NewDecoder(r.Body)
	err = decoder.Decode(&data)
	if err != nil {
		return errors.New("Failed to parse request body: " + err.Error()), tx
	}

	log.Println(data)

	if data.Type != "transaction.created" {
		return errors.New("Unexpected WebHook type: " + data.Type), tx
	}

	tx.SortCode = data.Data.CounterParty.SortCode
	tx.AccountNumber = data.Data.CounterParty.AccountNumber
	tx.Amount = data.Data.Amount
	tx.Currency = data.Data.Currency

	if tx.Amount <= 0 {
		return
	}

	if tx.SortCode == "" || tx.AccountNumber == "" {
		return errors.New("Counterparty data missing"), tx
	}

	if tx.Amount < 100 || tx.Amount > 5000 {
		return errors.New(fmt.Sprintf("Invalid amount. Send £1 - £50", OrderAmountPence/100.0)), tx
	}

	if data.Data.Currency != "GBP" {
		return errors.New("Wrong currency. Send GBP only"), tx
	}

	ethereumAddress, err := AccessCodeToEthereumAddress(data.Data.Description)
	if err != nil {
		return errors.New("Unknown access code"), tx
	}

	tx.EthAddress = eth.HexToAddress(ethereumAddress)

	return nil, tx
}

func Refund(tx Order, err error) error {

	if tx.SortCode == "" || tx.AccountNumber == "" || tx.Currency == "" {
		return errors.New("An error occurred but we do not have enough information to issue a refund: " + err.Error())
	}

	monzoClient.PostInfo("REFUND", fmt.Sprintf("%s %s %d %s %s", tx.SortCode, tx.AccountNumber, tx.Amount, tx.Currency, err.Error()))

	err2 := monzoClient.MoveToPot("refund", tx.Amount)

	if err2 != nil {
		return errors.New("Failed to deposit into Refund pot: " + err2.Error() + ". Original error: " + err.Error())
	}

	return err
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

	// Ignore outgoing transaction
	if order.Amount <= 0 {
		return nil
	}

	return logic.Fulfill(order)

	// // Try to buy ether
	// err, amount := coinbaseClient.BuyEther()
	// if err != nil {
	// 	// If buying ether fails, refund the user
	// 	return Refund(order, err)
	// }

	// // Try to send Ether to user
	// err = coinbaseClient.SendEther(amount, order.EthAddress)
	// if err != nil {
	// 	// If sending ether to user fails, refund
	// 	return Refund(order, err)
	// 	// TODO try to sell the ether on coinbase?
	// }

	// // Interaction with user is complete, now try to balance our internal books

	// err = monzoClient.MoveToPot("coinbase", EtherValueGBP*100)
	// if err != nil {
	// 	return err
	// }

	// err = monzoClient.MoveToPot("profit", ServiceChargeGBP*100)

	// return err
}
