FROM golang:latest

# Adding our root CA to make self-signed SSL cert valid.
ADD ca.crt /usr/local/share/ca-certificates
RUN update-ca-certificates

RUN go get golang.org/x/net/http2 github.com/spf13/pflag github.com/spf13/viper

COPY main.go /go/src/github.com/ernado/go-issue-25009/

RUN go install github.com/ernado/go-issue-25009

ENTRYPOINT ["/go/bin/go-issue-25009"]