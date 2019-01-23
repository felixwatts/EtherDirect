package main

const (
	PortHttp           = 80                          //8081
	PortHttps          = 443                         //8443
	HttpsRedirectRoot  = "https://etherdirect.co.uk" // "https://localhost:8443"
	HttpsCertificate   = "/etc/letsencrypt/live/etherdirect.co.uk/fullchain.pem"
	HttpsPrivateKey    = "/etc/letsencrypt/live/etherdirect.co.uk/privkey.pem"
	FileSystemRoot     = "./"
	AddressEtherDirect = "0xDaEF995931D6F00F56226b29ba70353327b21E00"
	ServiceChargeGBP   = 2
	EtherValueGBP      = 10
	OrderAmountPence   = (EtherValueGBP + ServiceChargeGBP) * 100
)
