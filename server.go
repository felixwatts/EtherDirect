package main

import (
	"errors"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
)

const PortHttp = 8081
const PortHttps = 8443
const HttpsRedirectRoot = "https://etherdirect.co.uk:443"
const HttpsCertificate = "/etc/letsencrypt/live/etherdirect.co.uk/fullchain.pem"
const HttpsPrivateKey = "/etc/letsencrypt/live/etherdirect.co.uk/privkey.pem"
const FileSystemRoot = "/var/www/etherdirect.co.uk/"
const AddressEtherDirect = "0xDaEF995931D6F00F56226b29ba70353327b21E00"
const MinOrderAmountPence = 500
const MaxOrderAmountPence = 500
const ServiceChargePence = 100

type MonzoTransaction struct {
	SortCode      string
	AccountNumber string
	AmountPence   uint
	EthAddress    common.Address
}

type IndexViewModel struct {
}

var templates = make(map[string]*template.Template)

func ParseMonzoTx(r *http.Request) (err error, tx MonzoTransaction) {
	return errors.New("Not implemented"), MonzoTransaction{}
}

func Refund(tx MonzoTransaction, msg string) error {
	// TODO
	return errors.New("Not implemented")
}

func BuyEther(amountPence uint) (err error, amountWei uint) {
	// TODO
	return errors.New("Not implemented"), 0
}

func SendEtherFromEtherDirectToUser(amountWei uint, to common.Address) error {
	return errors.New("Not implemented")
}

func SendEtherFromCoinbaseToEtherDirect(amountWei uint) error {
	return errors.New("Not implemented")
}

func HandleMonzoTransactionWebHook(w http.ResponseWriter, r *http.Request) {

	// TODO verify that this is really from Monzo

	// Parse and validate the incoming bank transfer
	err, tx := ParseMonzoTx(r)
	if err != nil {
		// If it's invalid refund the user
		Refund(tx, err.Error())
		return
	}

	// Try to buy ether
	err, amountWei := BuyEther(tx.AmountPence - ServiceChargePence)
	if err != nil {
		// If buying ether fails, refund the user
		Refund(tx, "Internal error")
		return
	}

	err = SendEtherFromEtherDirectToUser(amountWei, tx.EthAddress)
	if err != nil {
		// If sending ether to user fails, refund
		Refund(tx, "Internal error")
		// TODO try to sell the ether on coinbase?
		return
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
		filename := FileSystemRoot + tmpl + ".html"
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
