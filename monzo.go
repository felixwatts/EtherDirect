package main

import (
	"encoding/json"
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

var monzoClient = monzo.Client{}
var monzoLoginStateToken string

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

type MonzoAccessTokenGrant struct {
	AccessToken  string `json:"access_token"`
	ClientId     string `json:"client_id"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	UserId       string `json:"user_id"`
}

func monzoLoginHandler(w http.ResponseWriter, r *http.Request) {
	rand.Seed(time.Now().UnixNano())
	num := rand.Int()
	monzoLoginStateToken = strconv.Itoa(num)

	redirectUrl := fmt.Sprintf("https://auth.monzo.com/?client_id=%s&redirect_uri=%s&response_type=code&state=%s",
		os.Getenv("MonzoClientId"),
		url.QueryEscape("https://etherdirect.co.uk/monzo-oath-callback"),
		monzoLoginStateToken)

	http.Redirect(w, r, redirectUrl, http.StatusMovedPermanently)
}

func monzoLoginCallbackHandler(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	state := params["state"][0]
	if state != monzoLoginStateToken {
		log.Println("Invalid state in monzo oauth callback")
		return
	}

	code := params["code"][0]

	v := url.Values{}
	v.Set("grant_type", "authorization_code")
	v.Set("client_id", os.Getenv("MonzoClientId"))
	v.Set("client_secret", os.Getenv("MonzoClientSecret"))
	v.Set("redirect_uri", "https://etherdirect.co.uk/monzo-oath-callback")
	v.Set("code", code)

	getMonzoAccessToken(v)

	go refreshMonzoAccessToken()

	fmt.Fprint(w, "Successfully logged into Monzo")
}

func refreshMonzoAccessToken() {
	for {
		log.Println("Refreshing Monzo access token...")

		time.Sleep(1 * time.Hour)

		v := url.Values{}
		v.Set("grant_type", "refresh_token")
		v.Set("client_id", os.Getenv("MonzoClientId"))
		v.Set("client_secret", os.Getenv("MonzoClientSecret"))
		v.Set("refresh_token", monzoClient.RefreshToken)

		getMonzoAccessToken(v)
	}
}

func getMonzoAccessToken(params url.Values) {
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

	monzoClient = monzo.Client{
		AccessToken:  data.AccessToken,
		RefreshToken: data.RefreshToken,
		BaseURL:      "https://api.monzo.com/",
		UserID:       os.Getenv("MonzoUserId"),
	}

	log.Printf("Successfully logged into Monzo. Access token: %s Refresh Token: %s", monzoClient.AccessToken, monzoClient.RefreshToken)
}
