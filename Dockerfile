FROM golang:latest

RUN go get golang.org/x/net/http2 github.com/spf13/pflag github.com/spf13/viper

COPY main.go /go/src/github.com/ernado/go-issue-25009/

RUN go install github.com/ernado/go-issue-25009

ENTRYPOINT ["/go/bin/go-issue-25009"]