package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	monzo "github.com/tjvr/go-monzo"
)

type IMonzo interface {
	MoveToPot(potName string, amountPence int) error
}

type MonzoWebHookCounterParty struct {
	Name          string
	SortCode      string `json:"sort_code"`
	AccountNumber string `json:"account_number"`
}

type MonzoWebHookTransaction struct {
	AccountId    string `json:"account_id"`
	Description  string
	Amount       int
	Currency     string
	CounterParty MonzoWebHookCounterParty
}

type MonzoWebHook struct {
	Type string
	Data MonzoWebHookTransaction
}

type MonzoAccessTokenGrant struct {
	AccessToken  string `json:"access_token"`
	ClientId     string `json:"client_id"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	UserId       string `json:"user_id"`
}

type Monzo struct {
	client          monzo.Client
	oath2StateToken string
	isLoggedIn      bool
	nextDedupeId    int64
}

func (m *Monzo) HandleLogin(w http.ResponseWriter, r *http.Request) {
	rand.Seed(time.Now().UnixNano())
	num := rand.Int()
	m.oath2StateToken = strconv.Itoa(num)

	redirectUrl := fmt.Sprintf("https://auth.monzo.com/?client_id=%s&redirect_uri=%s&response_type=code&state=%s",
		os.Getenv("MonzoClientId"),
		url.QueryEscape("https://etherdirect.co.uk/monzo-oath-callback"),
		m.oath2StateToken)

	http.Redirect(w, r, redirectUrl, http.StatusMovedPermanently)
}

func (m *Monzo) HandleOauth2Callback(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	// TODO
	// state := params["state"][0]
	// if state != m.oath2StateToken {
	// 	log.Println("Invalid state in monzo oauth callback")
	// 	return
	// }

	code := params["code"][0]

	v := url.Values{}
	v.Set("grant_type", "authorization_code")
	v.Set("client_id", os.Getenv("MonzoClientId"))
	v.Set("client_secret", os.Getenv("MonzoClientSecret"))
	v.Set("redirect_uri", "https://etherdirect.co.uk/monzo-oath-callback")
	v.Set("code", code)

	m.GetAccessToken(v)

	go m.RefreshAccessToken()

	http.Redirect(w, r, "https://etherdirect.co.uk", http.StatusMovedPermanently)
}

func (m *Monzo) RefreshAccessToken() {
	for {
		time.Sleep(1 * time.Hour)

		log.Println("Refreshing Monzo access token...")

		v := url.Values{}
		v.Set("grant_type", "refresh_token")
		v.Set("client_id", os.Getenv("MonzoClientId"))
		v.Set("client_secret", os.Getenv("MonzoClientSecret"))
		v.Set("refresh_token", m.client.RefreshToken)

		m.GetAccessToken(v)
	}
}

func (m *Monzo) GetAccessToken(params url.Values) {
	rsp, err := http.PostForm("https://api.monzo.com/oauth2/token", params)

	if err != nil {
		panic(err.Error())
	}

	data := MonzoAccessTokenGrant{}
	decoder := json.NewDecoder(rsp.Body)
	err = decoder.Decode(&data)
	if err != nil {
		panic(err.Error())
	}

	m.client = monzo.Client{
		AccessToken:  data.AccessToken,
		RefreshToken: data.RefreshToken,
		BaseURL:      "https://api.monzo.com/",
		UserID:       os.Getenv("MonzoUserId"),
	}

	log.Printf("Successfully logged into Monzo. Access token: %s Refresh Token: %s", m.client.AccessToken, m.client.RefreshToken)
}

func (m *Monzo) PostError(err error) error {
	if !m.isLoggedIn {
		return errors.New("Not logged in to monzo")
	}

	return m.client.CreateFeedItem(&monzo.FeedItem{
		AccountID: os.Getenv("MonzoAccountId"),
		Title:     "ERROR",
		Body:      err.Error(),
		Type:      "basic",
		ImageURL:  "https://cdn0.iconfinder.com/data/icons/elasto-online-store/26/00-ELASTOFONT-STORE-READY_close-512.png",
	})
}

func (m *Monzo) PostInfo(heading string, msg string) error {
	if !m.isLoggedIn {
		return errors.New("Not logged in to monzo")
	}

	return m.client.CreateFeedItem(&monzo.FeedItem{
		AccountID: os.Getenv("MonzoAccountId"),
		Title:     heading,
		Body:      msg,
		Type:      "basic",
		ImageURL:  "https://cdn0.iconfinder.com/data/icons/elasto-online-store/26/00-STORE-37-512.png",
	})
}

func (m *Monzo) MoveToPot(potName string, amountPence int) error {
	potId := m.getPotId(potName)

	ddid := m.nextDedupeId
	m.nextDedupeId = m.nextDedupeId + 1

	_, err := m.client.Deposit(&monzo.DepositRequest{
		PotID:          potId,
		AccountID:      os.Getenv("MonzoAccountId"),
		Amount:         int64(amountPence),
		IdempotencyKey: strconv.FormatInt(ddid, 10),
	})

	if err != nil {
		return errors.New("Failed to deposit to Monzo pot " + potId + ": " + err.Error())
	}

	if amountPence < 0 {
		log.Printf("Withdrew %d from pot %s", -amountPence, potName)
	} else {
		log.Printf("Deposited %d into pot %s", amountPence, potName)
	}

	return nil
}

func (m *Monzo) GetBalance(potName string) (int64, error) {
	potId := m.getPotId(potName)
	p, e := m.client.Pot(potId)
	if e != nil {
		return 0, e
	}
	return p.Balance, nil
}

func (m *Monzo) getPotId(potName string) string {
	var potId string
	switch potName {
	case "coinbase":
		potId = os.Getenv("MonzoPotCoinbase")
	case "profit":
		potId = os.Getenv("MonzoPotCoinbase")
	case "refund":
		potId = os.Getenv("MonzoPotCoinbase")
	case "float":
		potId = os.Getenv("MonzoPotCoinbase")
	default:
		panic("unknown pot name")
	}
	return potId
}
