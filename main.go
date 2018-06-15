package main

import (
	"bytes"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"runtime"
	"sync"
	"time"

	"golang.org/x/net/http2"
)

var (
	client     = flag.Bool("client", false, "client")
	addr       = flag.String("addr", "0.0.0.0:80", "addr")
	url        = flag.String("url", "https://nginx", "url")
	concurrent = flag.Bool("concurrent", true, "use one client for all goroutines concurrently")
)

func newClient() *http.Client {
	var dialer = &net.Dialer{
		Timeout: 10 * time.Second,
	}
	var netTransport = &http.Transport{
		DialContext:         dialer.DialContext,
		TLSHandshakeTimeout: 10 * time.Second,
		MaxIdleConns:        20,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     60 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	http2.ConfigureTransport(netTransport)
	return &http.Client{
		Timeout:   time.Second * 30,
		Transport: netTransport,
	}
}

func startServer() {
	fmt.Println("listening on", *addr)
	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		io.Copy(ioutil.Discard, request.Body)
		request.Body.Close()
		writer.WriteHeader(http.StatusOK)
	})
	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatal(err)
	}
}

func startClient() {
	fmt.Println("client to", *url)
	wg := new(sync.WaitGroup)
	var cNetClient = newClient()
	for i := 0; i < runtime.GOMAXPROCS(-1); i++ {
		wg.Add(1)
		go func(id int) {
			netClient := cNetClient
			if !*concurrent {
				netClient = newClient()
			}
			for {
				buf := make([]byte, 10000)
				req, err := http.NewRequest(http.MethodPost, *url+fmt.Sprintf("/%d", id), bytes.NewReader(buf))
				if err != nil {
					log.Fatal(err)
				}
				res, err := netClient.Do(req)
				if err != nil {
					fmt.Println("err", err)
					continue
				}
				// Ensuring connection reuse.
				io.Copy(ioutil.Discard, res.Body)
				res.Body.Close()

				if res.StatusCode != http.StatusOK {
					fmt.Println(res.Status)
				}
			}
		}(i)
	}
	wg.Wait()
}

func main() {
	flag.Parse()
	if !*client {
		startServer()
	}
	startClient()
}
