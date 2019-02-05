package main

import (
	"fmt"
	"log"
)

type Logic struct {
	etherBalance float64
	coinbase     ICoinbase
	monzo        IMonzo
}

func (l *Logic) Fulfill(o Order) error {

	// get ether price
	etherPrice, err := l.coinbase.GetEtherPrice()
	if err != nil {
		return err
	}

	// get ether amount to fulfill E
	commission := int(float64(o.Amount) * 0.15)
	etherValueGbp := float64(o.Amount-commission) / 100.0
	etherAmount := etherValueGbp / etherPrice

	log.Printf("Amount O: %d, Commission: %d, Price: %f, Value: %f, Amount E: %f",
		o.Amount, commission, etherPrice, etherValueGbp, etherAmount)

	// while E > ether balance
	for etherAmount > l.etherBalance {

		log.Printf("Balance E: %f, Buying Ether", l.etherBalance)

		// buy £10 worth ether
		err, filledSize := l.coinbase.BuyEther()
		if err != nil {
			return err
		}

		// increase ether balance
		l.etherBalance += filledSize

		// send 10 from float to coinbase
		l.monzo.MoveToPot("float", -1000)
		l.monzo.MoveToPot("coinbase", 1000)
	}

	log.Printf("Balance E: %f, Sending Ether", l.etherBalance)

	// send ether to user
	etherAmountStr := fmt.Sprintf("%f", etherAmount)
	//log.Printf("Send %s ether to user", etherAmountStr)
	l.coinbase.SendEther(etherAmountStr, o.EthAddress)

	// adjust ether balance
	l.etherBalance -= etherAmount

	// add (payment - commission) to float
	l.monzo.MoveToPot("float", o.Amount-commission)

	// add commission to profit
	l.monzo.MoveToPot("profit", commission)

	log.Printf("Balance E: %f", l.etherBalance)

	return nil
}
