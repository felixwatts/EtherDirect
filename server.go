package main

import (
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	coinbase "github.com/preichenberger/go-gdax"
	monzo "github.com/tjvr/go-monzo"
)

var templates = make(map[string]*template.Template)
var coinbaseClient = coinbase.NewClient(CoinbaseSecret, CoinbaseKey, CoinbasePassphrase)
var monzoClient = monzo.Client{
	BaseURL:     "https://api.monzo.com",
	AccessToken: MonzoAccessToken,
}
var nextDedupeId int64 = 0

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

func monzoWebhookHandler(w http.ResponseWriter, r *http.Request) {
	HandleError(ProcessOrder(w, r))
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
