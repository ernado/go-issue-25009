package main

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"
	"sync/atomic"

	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	"golang.org/x/net/http2"
)

func newClient() *http.Client {
	var netTransport = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	if err := http2.ConfigureTransport(netTransport); err != nil {
		log.Fatalln("failed to configure http2:", err)
	}
	c := &http.Client{
		Transport: netTransport,
	}
	if !viper.GetBool("tls_skip_verify") {
		c.Transport = http.DefaultTransport
	}
	if viper.GetBool("http2_transport") {
		c.Transport = &http2.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}
	return c
}

func startServer() {
	fmt.Println("listening on", viper.GetString("addr"))
	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		io.Copy(ioutil.Discard, request.Body)
		request.Body.Close()
		writer.WriteHeader(http.StatusOK)
	})
	if err := http.ListenAndServe(viper.GetString("addr"), nil); err != nil {
		log.Fatal(err)
	}
}

func startClient() {
	var (
		url        = viper.GetString("url")
		wg         = new(sync.WaitGroup)
		c          = newClient()
		jobs       = viper.GetInt("jobs")
		request    = viper.GetInt64("requests")
		concurrent = viper.GetBool("concurrent")
		setGetBody = viper.GetBool("body")
	)
	fmt.Println("client to", url)
	fmt.Printf("will do ~%d requests with %d concurrency\n", request, jobs)
	if setGetBody {
		fmt.Println("will set req.GetBody explicitly")
	}
	if concurrent {
		fmt.Println("using one client for all goroutines")
	} else {
		fmt.Println("using separate client per goroutine")
	}
	if viper.GetBool("http2_transport") {
		fmt.Println("using http2 transport directly")
	}
	if !viper.GetBool("tls_skip_verify") {
		fmt.Println("using http.DefaultTransport without InsecureSkipVerify")
	}
	var count, failed int64
	for i := 0; i < jobs; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			httpClient := c
			if !concurrent {
				httpClient = newClient()
			}
			for {
				if current := atomic.AddInt64(&count, 1); current >= request {
					break
				}
				buf := make([]byte, 100)
				req, err := http.NewRequest(http.MethodPost, url+fmt.Sprintf("/%d", id), bytes.NewReader(buf))
				if err != nil {
					log.Fatal(err)
				}
				if setGetBody {
					req.ContentLength = int64(len(buf))
					req.GetBody = func() (io.ReadCloser, error) {
						return ioutil.NopCloser(bytes.NewReader(buf)), nil
					}
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
	flag.Bool("client", false, "client")
	flag.String("addr", "0.0.0.0:80", "addr")
	flag.String("url", "https://nginx", "url")
	flag.Bool("concurrent", true, "use one client for all goroutines concurrently")
	flag.Int("jobs", 6, "number of concurrent requests")
	flag.Int64("requests", 100, "number of total requests")
	flag.Bool("body", false, "set request.GetBody")
	flag.Parse()
	viper.SetDefault("http2_transport", false)
	viper.SetDefault("tls_skip_verify", true)
	viper.AutomaticEnv()
	viper.BindPFlags(flag.CommandLine)
	if !viper.GetBool("client") {
		startServer()
	}
	startClient()
}
