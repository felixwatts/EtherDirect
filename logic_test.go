package main

import (
	"math"
	"strconv"
	"testing"

	eth "github.com/ethereum/go-ethereum/common"
)

type MockMonzo struct {
	Pots    map[string]int
	Balance int
}

type MockCoinbase struct {
	EtherPrice  float64
	BalanceGbp  float64
	BalanceEth  float64
	EthAccounts map[string]float64
}

func (m *MockMonzo) MoveToPot(potName string, amountPence int) error {
	m.Balance -= amountPence
	m.Pots[potName] += amountPence
	return nil
}

func (c *MockCoinbase) BuyEther() (err error, filledSize float64) {
	c.BalanceGbp -= EtherValueGBP
	filledSize = EtherValueGBP / c.EtherPrice
	c.BalanceEth += filledSize
	return nil, filledSize
}

func (c *MockCoinbase) SendEther(amount string, to eth.Address) error {
	amountFloat, _ := strconv.ParseFloat(amount, 64)
	c.BalanceEth -= amountFloat
	c.EthAccounts[to.String()] += amountFloat
	return nil
}

func (c *MockCoinbase) GetEtherPrice() (float64, error) {
	return c.EtherPrice, nil
}

func TestOrderSmallerThanBalance(t *testing.T) {
	Do(t, 10000, 1.0, 0, 1500, 8500, 0.15, 0.85)
}

func TestOrderLargerThanBalance(t *testing.T) {
	Do(t, 1000, 0.0, 1000, 150, -150, 0.015, 0.085)
}

func TestOrderMuchLargerThanBalance(t *testing.T) {
	Do(t, 10000, 0.0, 9000, 1500, -500, 0.05, 0.85)
}

func Do(
	t *testing.T, orderSizePence int,
	balanceEth float64,
	expectedCoinbasePot int,
	expectedProfitPot int,
	expectedFloatPot int,
	expectedEthBalance float64,
	expectedCustomerEthBalance float64) {

	monzo := MockMonzo{
		Pots:    make(map[string]int),
		Balance: orderSizePence,
	}

	coinbase := MockCoinbase{
		BalanceEth:  balanceEth,
		BalanceGbp:  0,
		EthAccounts: make(map[string]float64),
		EtherPrice:  100,
	}

	subject := Logic{
		coinbase:     &coinbase,
		monzo:        &monzo,
		etherBalance: balanceEth,
	}

	order := Order{
		AccountNumber: "123456789",
		Amount:        orderSizePence,
		Currency:      "GBP",
		EthAddress:    eth.HexToAddress("0x52Ec249dD2eEc428b1E2f389c7d032caF5D1a238"),
		SortCode:      "123456",
	}

	subject.Fulfill(order)

	if monzo.Pots["coinbase"] != expectedCoinbasePot {
		t.Error("coinbase pot")
	}

	if monzo.Pots["profit"] != expectedProfitPot {
		t.Error("profit pot")
	}

	if monzo.Pots["float"] != expectedFloatPot {
		t.Errorf("float pot %d", monzo.Pots["float"])
	}

	if monzo.Balance != 0 {
		t.Errorf("monzo balance %d", monzo.Balance)
	}

	if math.Abs(subject.etherBalance-expectedEthBalance) > 0.00001 {
		t.Errorf("logic ether balance %f", subject.etherBalance)
	}

	if math.Abs(coinbase.BalanceEth-expectedEthBalance) > 0.00001 {
		t.Error("coinbase ether balance")
	}

	if math.Abs(coinbase.EthAccounts["0x52Ec249dD2eEc428b1E2f389c7d032caF5D1a238"]-expectedCustomerEthBalance) > 0.00001 {
		t.Error("customer eth balance")
	}
}
