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

	"github.com/ethereum/go-ethereum/common"
)

const PortHttp = 8081
const PortHttps = 8443
const HttpsRedirectRoot = "https://localhost:8443"
const HttpsCertificate = "/etc/letsencrypt/live/etherdirect.co.uk/fullchain.pem"
const HttpsPrivateKey = "/etc/letsencrypt/live/etherdirect.co.uk/privkey.pem"
const FileSystemRoot = "./"
const AddressEtherDirect = "0xDaEF995931D6F00F56226b29ba70353327b21E00"
const OrderAmountPence = 500
const ServiceChargePence = 100
const EtherValuePence = OrderAmountPence - ServiceChargePence

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
	EthAddress    common.Address
}

func (o Order) String() string {
	return fmt.Sprintf("{ %s %s %s %d %s }", o.SortCode, o.AccountNumber, o.Currency, o.Amount, o.EthAddress.Hex())
}

type IndexViewModel struct {
}

var templates = make(map[string]*template.Template)

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
		return errors.New(fmt.Sprintf("Wrong amount. Send %dp exactly", OrderAmountPence)), tx
	}

	if data.Data.Currency != "GBP" {
		return errors.New("Wrong currency. Send GBP only"), tx
	}

	// TODO validate address

	if !IsValidAddress(data.Data.Description) {
		return errors.New("Reference field of transfer must contain a valid Ethereum address"), tx
	}

	tx.EthAddress = common.HexToAddress(data.Data.Description)

	return nil, tx
}

func Refund(tx Order, msg string) error {
	// TODO

	if tx.SortCode == "" || tx.AccountNumber == "" || tx.Currency == "" {
		log.Println("ERROR: An error occurred but we do not have enough information to issue a refund: " + msg)
		return nil
	}

	log.Printf("TODO Refund %d (%s) to %s-%s. %s", tx.Amount, tx.Currency, tx.SortCode, tx.AccountNumber, msg)
	return errors.New("Not implemented")
}

func BuyEtherOnCoinbase() (err error, amountWei uint) {
	log.Printf("Buy %d worth of ETH on coinbase", EtherValuePence)

	// TODO

	return nil, 42
}

func SendEtherFromEtherDirectToUser(amountWei uint, to common.Address) error {

	log.Printf("Send %d wei from %s to %s", amountWei, AddressEtherDirect, to.Hex())

	// TODO

	return nil
}

func SendGbpFromMonzoToCoinbase() error {

	log.Printf("Send %dp from Monzo to Coinbase", EtherValuePence)

	// TODO

	return nil
}

func SendEtherFromCoinbaseToEtherDirect(amountWei uint) error {
	log.Printf("Send %d wei from Coinbase to %s", amountWei, AddressEtherDirect)

	// TODO

	return nil
}

func HandleMonzoTransactionWebHook(w http.ResponseWriter, r *http.Request) {

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO verify that this is really from Monzo

	log.Println("New order")

	// Parse and validate the incoming bank transfer
	err, order := ParseOrder(r)
	log.Println(order)
	if err != nil {
		// If it's invalid refund the user
		Refund(order, err.Error())
		return
	}

	// Try to buy ether
	err, amountWei := BuyEtherOnCoinbase()
	if err != nil {
		// If buying ether fails, refund the user
		Refund(order, "Internal error")
		return
	}

	err = SendEtherFromEtherDirectToUser(amountWei, order.EthAddress)
	if err != nil {
		// If sending ether to user fails, refund
		Refund(order, "Internal error")
		// TODO try to sell the ether on coinbase?
		return
	}

	err = SendGbpFromMonzoToCoinbase()
	if err != nil {
		log.Println(err)
	}

	err = SendEtherFromCoinbaseToEtherDirect(amountWei)
	if err != nil {
		log.Println(err)
	}
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
