package main

import (
  "flag"
  "log"
  "strconv"
  "time"
  "crypto/tls"
  "net/http"
  "github.com/nokia/danm/pkg/netadmit"
)

func main() {
  cert := flag.String("tls-cert-bundle", "", "File containing the x509 Certificate for HTTPS. (CA cert, if any, concatenated after server cert).")
  key := flag.String("tls-private-key-file", "", "File containing the x509 private key matching --tls-cert-bundle.")
  port := flag.Int("bind-port", 443, "The port on which to serve. Default is 443.")
  address := flag.String("bind-address", "0.0.0.0", "The IP address on which to listen. Default is all interfaces.")
  flag.Parse()
  if cert == nil || key == nil {
    log.Println("ERROR: Configuring TLS is mandatory, --tls-cert-bundle and --tls-private-key-file cannot be empty!")
    return
  }
  tlsConf, err := tls.LoadX509KeyPair(*cert, *key)
  if err != nil {
    log.Println("ERROR: TLS configuration could not be initialized, because:" + err.Error())
    return
  }
  http.HandleFunc("/webhook", netadmit.ValidateNetwork)
  server := &http.Server{
    Addr:         *address + ":" + strconv.Itoa(*port),
    TLSConfig:    &tls.Config{Certificates: []tls.Certificate{tlsConf}},
    ReadTimeout:  5 * time.Second,
    WriteTimeout: 5 * time.Second,
  }
  server.ListenAndServeTLS("", "")
}