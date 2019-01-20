package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"time"

	eth "github.com/ethereum/go-ethereum/common"
	coinbase "github.com/preichenberger/go-gdax"
	"github.com/tjvr/go-monzo"
)

const PortHttp = 8081
const PortHttps = 8443
const HttpsRedirectRoot = "https://localhost:8443"
const HttpsCertificate = "/etc/letsencrypt/live/etherdirect.co.uk/fullchain.pem"
const HttpsPrivateKey = "/etc/letsencrypt/live/etherdirect.co.uk/privkey.pem"
const FileSystemRoot = "./"
const AddressEtherDirect = "0xDaEF995931D6F00F56226b29ba70353327b21E00"
const ServiceChargeGBP = 2
const EtherValueGBP = 10
const OrderAmountPence = (EtherValueGBP + ServiceChargeGBP) * 100

var templates = make(map[string]*template.Template)
var coinbaseClient = coinbase.NewClient(CoinbaseSecret, CoinbaseKey, CoinbasePassphrase)
var monzoClient = monzo.Client{
	BaseURL:     "https://api.monzo.com",
	AccessToken: MonzoAccessToken,
}
var nextDedupeId int64 = 0

type MonzoWebHookCounterParty struct {
	Name           string
	Sort_Code      string
	Account_Number string
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

type IndexViewModel struct {
}

func PostMonzoFeedError(err error) error {
	return monzoClient.CreateFeedItem(&monzo.FeedItem{
		AccountID: MonzoAccountId,
		Title:     "ERROR",
		Body:      err.Error(),
		Type:      "basic",
		ImageURL:  "https://cdn0.iconfinder.com/data/icons/elasto-online-store/26/00-ELASTOFONT-STORE-READY_close-512.png",
	})
}

func PostMonzoFeedInfo(heading string, msg string) error {
	return monzoClient.CreateFeedItem(&monzo.FeedItem{
		AccountID: MonzoAccountId,
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
		AccountID:      MonzoAccountId,
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

	tx.SortCode = data.Data.CounterParty.Sort_Code
	tx.AccountNumber = data.Data.CounterParty.Account_Number
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

	err2 := DepositToMonzoPot(MonzoPotIdRefund, tx.Amount)

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
	return DepositToMonzoPot(MonzoPotIdCoinbase, EtherValueGBP*100)
}

func HandleMonzoTransactionWebHook(w http.ResponseWriter, r *http.Request) {
	HandleError(handleMonzoTransactionWebHook(w, r))
}

func handleMonzoTransactionWebHook(w http.ResponseWriter, r *http.Request) error {

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

	err = DepositToMonzoPot(MonzoPotIdProfit, ServiceChargeGBP*100)

	return err
}

func logAndDelegate(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.URL.Path, r.RemoteAddr, r.Referer(), r.UserAgent())
		handler.ServeHTTP(w, r)
	})
}

func faviconHandler(w http.ResponseWriter, r *http.Request) {
	// TODO
	http.NotFound(w, r)
}

func redirectToHttpsHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, HttpsRedirectRoot+r.RequestURI, http.StatusMovedPermanently)
}

func defaultHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/", http.StatusFound)
}

func renderTemplate(tmpl string, model interface{}, w http.ResponseWriter) {

	err := templates[tmpl].Execute(w, model)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	vm := IndexViewModel{}

	renderTemplate("index", vm, w)
}

func init() {
	for _, tmpl := range []string{"index"} {
		filename := FileSystemRoot + "html/" + tmpl + ".html"
		t, err := template.ParseFiles(filename)
		if err != nil {
			panic(err)
		}

		templates[tmpl] = t
	}

	nextDedupeId = time.Now().Unix()
}

func main() {

	httpsMux := http.NewServeMux()

	httpsMux.HandleFunc("/favicon.ico", faviconHandler)
	httpsMux.HandleFunc("/", indexHandler)
	httpsMux.HandleFunc("/monzo-"+WebHookSecretUrlPart, HandleMonzoTransactionWebHook)
	httpsMux.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir(FileSystemRoot+"js"))))
	httpsMux.Handle("/css/", http.StripPrefix("/css/", http.FileServer(http.Dir(FileSystemRoot+"css"))))

	httpMux := http.NewServeMux()

	httpMux.Handle("/.well-known/acme-challenge/", http.StripPrefix("/.well-known/acme-challenge/", http.FileServer(http.Dir(FileSystemRoot+".well-known/acme-challenge"))))
	httpMux.Handle("/img/", http.StripPrefix("/img/", http.FileServer(http.Dir(FileSystemRoot+"img"))))
	httpMux.HandleFunc("/", redirectToHttpsHandler)

	go http.ListenAndServe(":"+strconv.Itoa(PortHttp), logAndDelegate(httpMux))
	log.Fatal(http.ListenAndServeTLS(":"+strconv.Itoa(PortHttps), HttpsCertificate, HttpsPrivateKey, logAndDelegate(httpsMux)))
}
