package main

import (
	"bytes"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"
	"sync/atomic"

	"golang.org/x/net/http2"
)

var (
	client     = flag.Bool("client", false, "client")
	addr       = flag.String("addr", "0.0.0.0:80", "addr")
	url        = flag.String("url", "https://nginx", "url")
	concurrent = flag.Bool("concurrent", true, "use one client for all goroutines concurrently")
	jobs       = flag.Int("j", 6, "number of concurrent requests")
	requests   = flag.Int64("requests", 100, "number of total requests")
)

func newClient() *http.Client {
	var netTransport = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	if err := http2.ConfigureTransport(netTransport); err != nil {
		log.Fatalln("failed to configure http2:")
	}
	return &http.Client{
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
	fmt.Printf("will do ~%d requests with %d concurrency\n", *requests, *jobs)
	var (
		wg = new(sync.WaitGroup)
		c  = newClient()
	)
	var count, failed int64
	for i := 0; i < *jobs; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			httpClient := c
			if !*concurrent {
				httpClient = newClient()
			}
			for {
				if current := atomic.AddInt64(&count, 1); current >= *requests {
					break
				}
				buf := make([]byte, 100)
				req, err := http.NewRequest(http.MethodPost, *url+fmt.Sprintf("/%d", id), bytes.NewReader(buf))
				if err != nil {
					log.Fatal(err)
				}
				res, err := httpClient.Do(req)
				if err != nil {
					atomic.AddInt64(&failed, 1)
					fmt.Println("err", err)
					continue
				}
				// Ensuring connection reuse.
				io.Copy(ioutil.Discard, res.Body)
				res.Body.Close()

				if res.StatusCode != http.StatusOK {
					atomic.AddInt64(&failed, 1)
					fmt.Println(res.Status)
				}
			}
		}(i)
	}
	wg.Wait()
	if failed == 0 {
		fmt.Println("OK")
	} else {
		fmt.Println("FAILED:", failed)
		os.Exit(2)
	}
}

func main() {
	flag.Parse()
	if !*client {
		startServer()
	}
	startClient()
}
