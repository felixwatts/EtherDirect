package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

var templates = make(map[string]*template.Template)
var monzoClient = Monzo{
	nextDedupeId: time.Now().Unix(),
}
var coinbaseClient = Coinbase{}
var logic = Logic{
	coinbase: &coinbaseClient,
	monzo:    &monzoClient,
}

var nextAccessCode uint = 0

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

type GetAccessCodeResponse struct {
	Error      string `json:"error"`
	AccessCode string `json:"access_code"`
}

func getAccessCodeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.NotFound(w, r)
		return
	}

	address := r.FormValue("ethereum-address")

	response := GetAccessCodeResponse{}

	if IsValidAddress(address) {

		accessCode := time.Now().Unix()

		filename := fmt.Sprintf("%saccess-codes/%d.txt", FileSystemRoot, accessCode)

		err := ioutil.WriteFile(filename, []byte(address), 0644)

		if err != nil {
			log.Println(err.Error())
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		response.AccessCode = fmt.Sprintf("%d", accessCode)

		log.Printf("Issued access code %d for address %s", accessCode, address)

	} else {
		log.Println("Cannot issue access code: Invalid ethereum address")
		response.Error = "Invalid ethereum address"
	}

	json, err := json.Marshal(response)

	if err != nil {
		log.Println(err.Error())
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Write(json)
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

	coinbaseClient.Init()
}

func main() {

	httpsMux := http.NewServeMux()

	httpsMux.HandleFunc("/favicon.ico", faviconHandler)
	httpsMux.HandleFunc("/", indexHandler)
	httpsMux.HandleFunc("/get-access-code", getAccessCodeHandler)
	httpsMux.HandleFunc("/monzo-"+os.Getenv("WebHookSecretUrlPart"), monzoWebhookHandler)
	httpsMux.HandleFunc("/monzo-login", monzoClient.HandleLogin)
	httpsMux.HandleFunc("/monzo-oath-callback", monzoClient.HandleOauth2Callback)
	httpsMux.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir(FileSystemRoot+"js"))))
	httpsMux.Handle("/css/", http.StripPrefix("/css/", http.FileServer(http.Dir(FileSystemRoot+"css"))))
	httpsMux.Handle("/img/", http.StripPrefix("/img/", http.FileServer(http.Dir(FileSystemRoot+"img"))))

	httpMux := http.NewServeMux()

	httpMux.Handle("/.well-known/acme-challenge/", http.StripPrefix("/.well-known/acme-challenge/", http.FileServer(http.Dir(FileSystemRoot+".well-known/acme-challenge"))))
	httpMux.HandleFunc("/", redirectToHttpsHandler)

	go http.ListenAndServe(":"+strconv.Itoa(PortHttp), logAndDelegate(httpMux))
	log.Fatal(http.ListenAndServeTLS(":"+strconv.Itoa(PortHttps), HttpsCertificate, HttpsPrivateKey, logAndDelegate(httpsMux)))
}
