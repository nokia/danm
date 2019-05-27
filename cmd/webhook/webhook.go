package main

import (
  "flag"
  "log"
  "strconv"
  "time"
  "crypto/tls"
  "net/http"
  "github.com/nokia/danm/pkg/admit"
)

func main() {
  cert := flag.String("tls-cert-bundle", "", "File containing the x509 Certificate for HTTPS. (CA cert, if any, concatenated after server cert).")
  key := flag.String("tls-private-key-file", "", "File containing the x509 private key matching --tls-cert-bundle.")
  port := flag.Int("bind-port", 8443, "The port on which to serve. Default is 8443.")
  address := flag.String("bind-address", "", "The IP address on which to listen. Default is all interfaces.")
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
  http.HandleFunc("/netvalidation", admit.ValidateNetwork)
  http.HandleFunc("/confvalidation", admit.ValidateTenantConfig)
  server := &http.Server{
    Addr:         *address + ":" + strconv.Itoa(*port),
    TLSConfig:    &tls.Config{Certificates: []tls.Certificate{tlsConf}},
    ReadTimeout:  5 * time.Second,
    WriteTimeout: 5 * time.Second,
  }
  log.Println("INFO:DANM webhook is about to start listening on " + *address + ":" + strconv.Itoa(*port))
  err = server.ListenAndServeTLS("", "")
  log.Fatal(err)
}