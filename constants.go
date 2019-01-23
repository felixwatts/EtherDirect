package main

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
