FROM golang:rc

# Adding our root CA to make self-signed SSL cert valid.
ADD certs/ca.crt /usr/local/share/ca-certificates
RUN update-ca-certificates

WORKDIR /root
COPY go.mod go.sum ./
RUN go mod download

COPY main.go ./
RUN go install -race .

ENTRYPOINT ["/go/bin/go-issue-25009"]
