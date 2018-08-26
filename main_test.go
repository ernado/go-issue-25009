package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/summerwind/h2spec/config"
	"github.com/summerwind/h2spec/spec"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

// localhostCert is a PEM-encoded TLS cert with SAN IPs
// "127.0.0.1" and "[::1]", expiring at Jan 29 16:00:00 2084 GMT.
// generated from src/crypto/tls:
// go run generate_cert.go  --rsa-bits 1024 --host 127.0.0.1,::1,example.com --ca --start-date "Jan 1 00:00:00 1970" --duration=1000000h
var localhostCert = []byte(`-----BEGIN CERTIFICATE-----
MIICEzCCAXygAwIBAgIQMIMChMLGrR+QvmQvpwAU6zANBgkqhkiG9w0BAQsFADAS
MRAwDgYDVQQKEwdBY21lIENvMCAXDTcwMDEwMTAwMDAwMFoYDzIwODQwMTI5MTYw
MDAwWjASMRAwDgYDVQQKEwdBY21lIENvMIGfMA0GCSqGSIb3DQEBAQUAA4GNADCB
iQKBgQDuLnQAI3mDgey3VBzWnB2L39JUU4txjeVE6myuDqkM/uGlfjb9SjY1bIw4
iA5sBBZzHi3z0h1YV8QPuxEbi4nW91IJm2gsvvZhIrCHS3l6afab4pZBl2+XsDul
rKBxKKtD1rGxlG4LjncdabFn9gvLZad2bSysqz/qTAUStTvqJQIDAQABo2gwZjAO
BgNVHQ8BAf8EBAMCAqQwEwYDVR0lBAwwCgYIKwYBBQUHAwEwDwYDVR0TAQH/BAUw
AwEB/zAuBgNVHREEJzAlggtleGFtcGxlLmNvbYcEfwAAAYcQAAAAAAAAAAAAAAAA
AAAAATANBgkqhkiG9w0BAQsFAAOBgQCEcetwO59EWk7WiJsG4x8SY+UIAA+flUI9
tyC4lNhbcF2Idq9greZwbYCqTTTr2XiRNSMLCOjKyI7ukPoPjo16ocHj+P3vZGfs
h1fIw3cSS2OolhloGw/XM6RWPWtPAlGykKLciQrBru5NAPvCMsb/I1DAceTiotQM
fblo6RBxUQ==
-----END CERTIFICATE-----`)

// localhostKey is the private key for localhostCert.
var localhostKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIICXgIBAAKBgQDuLnQAI3mDgey3VBzWnB2L39JUU4txjeVE6myuDqkM/uGlfjb9
SjY1bIw4iA5sBBZzHi3z0h1YV8QPuxEbi4nW91IJm2gsvvZhIrCHS3l6afab4pZB
l2+XsDulrKBxKKtD1rGxlG4LjncdabFn9gvLZad2bSysqz/qTAUStTvqJQIDAQAB
AoGAGRzwwir7XvBOAy5tM/uV6e+Zf6anZzus1s1Y1ClbjbE6HXbnWWF/wbZGOpet
3Zm4vD6MXc7jpTLryzTQIvVdfQbRc6+MUVeLKwZatTXtdZrhu+Jk7hx0nTPy8Jcb
uJqFk541aEw+mMogY/xEcfbWd6IOkp+4xqjlFLBEDytgbIECQQDvH/E6nk+hgN4H
qzzVtxxr397vWrjrIgPbJpQvBsafG7b0dA4AFjwVbFLmQcj2PprIMmPcQrooz8vp
jy4SHEg1AkEA/v13/5M47K9vCxmb8QeD/asydfsgS5TeuNi8DoUBEmiSJwma7FXY
fFUtxuvL7XvjwjN5B30pNEbc6Iuyt7y4MQJBAIt21su4b3sjXNueLKH85Q+phy2U
fQtuUE9txblTu14q3N7gHRZB4ZMhFYyDy8CKrN2cPg/Fvyt0Xlp/DoCzjA0CQQDU
y2ptGsuSmgUtWj3NM9xuwYPm+Z/F84K6+ARYiZ6PYj013sovGKUFfYAqVXVlxtIX
qyUBnu3X9ps8ZfjLZO7BAkEAlT4R5Yl6cGhaJQYZHOde3JEMhNRcVFMO8dJDaFeo
f9Oeos0UUothgiDktdQHxdNEwLjQf7lJJBzV+5OtwswCWA==
-----END RSA PRIVATE KEY-----`)

func startTestServer(t *testing.T, listener net.Listener) {
	var (
		requestID  int
		requestMux sync.Mutex
	)
	for i := 0; i < runtime.GOMAXPROCS(-1); i++ {
		go func() {
			for {
				conn, acceptErr := listener.Accept()
				if acceptErr != nil {
					t.Error(acceptErr)
					return
				}
				h2Conf := &config.Config{
					Verbose: true,
					Timeout: time.Second * 2,
				}
				h2Conn, err := spec.Accept(h2Conf, conn)
				if err != nil {
					t.Errorf("server: failed to accept: %v", err)
					continue
				}
				if err = h2Conn.Handshake(); err != nil {
					if err == io.EOF {
						break
					}
					t.Errorf("http2: server: failed to handshake: %v", err)
					continue
				}
				req, err := h2Conn.ReadRequest()
				if err != nil {
					t.Errorf("http2: server: failed to read request: %v", err)
					continue
				}
				t.Logf("server: read request: %s", req.Headers)

				requestMux.Lock()
				requestID++
				shouldGoAway := requestID > 1
				if shouldGoAway {
					requestID = 0
				}
				requestMux.Unlock()
				if shouldGoAway {
					// Simulating the nginx behaviour.
					h2Conn.WriteGoAway(0, http2.ErrCodeNo, []byte("nope"))
					h2Conn.Close()
					continue
				}

				ev, ok := h2Conn.WaitEventByType(spec.EventDataFrame)
				if !ok {
					panic("no data frame")
				}
				d := ev.(spec.DataFrameEvent)
				t.Logf("server: got length: %d", d.Length)
				if d.Length == 0 {
					// Got zero-length DATA, returning with error.
					headers := []hpack.HeaderField{
						spec.HeaderField(":status", "400"),
					}
					hp := http2.HeadersFrameParam{
						StreamID:      d.StreamID,
						EndStream:     true,
						EndHeaders:    true,
						BlockFragment: h2Conn.EncodeHeaders(headers),
					}
					h2Conn.WriteHeaders(hp)
					h2Conn.WriteData(req.StreamID, true, []byte("error"))
				} else {
					h2Conn.WriteSuccessResponse(req.StreamID, h2Conf)
				}
				h2Conn.WriteGoAway(0, http2.ErrCodeNo, []byte("just closing"))
				h2Conn.Close()
			}
		}()
	}
}

func getTLSConfig(t *testing.T) *tls.Config {
	tlsCert, err := tls.X509KeyPair(localhostCert, localhostKey)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	certPool := x509.NewCertPool()
	certPool.AddCert(certificate)
	return &tls.Config{
		MinVersion:               tls.VersionTLS12,
		CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
		PreferServerCipherSuites: true,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
		NextProtos:   []string{http2.NextProtoTLS},
		RootCAs:      certPool,
		Certificates: []tls.Certificate{tlsCert},
	}
}

func TestIssue25009(t *testing.T) {
	tlsConfig := getTLSConfig(t)
	listener, listenErr := tls.Listen("tcp4", "127.0.0.1:0", tlsConfig)
	if listenErr != nil {
		t.Fatal(listenErr)
	}
	startTestServer(t, listener)
	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	http2.ConfigureTransport(transport)
	client := &http.Client{
		Transport: transport,
	}
	var (
		addr     = "https://" + listener.Addr().String()
		wg       = new(sync.WaitGroup)
		reqCount int32
	)
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			for {
				current := atomic.AddInt32(&reqCount, 1)
				if current > 2 {
					break
				}
				buf := new(bytes.Buffer)
				fmt.Fprint(buf, current)
				req, err := http.NewRequest(http.MethodPost, addr, buf)
				if err != nil {
					log.Fatal(err)
				}
				res, err := client.Do(req)
				if err != nil {
					t.Errorf("client: %v", err)
					continue
				}
				io.Copy(ioutil.Discard, res.Body)
				res.Body.Close()
				if res.StatusCode != http.StatusOK {
					t.Errorf("client: failed: code=%d %03d", res.StatusCode, current)
				} else {
					t.Logf("client: ok: %03d", current)
				}
			}
		}()
	}
	wg.Wait()
}
