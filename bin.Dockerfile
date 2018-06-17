FROM debian:latest

RUN apt-get update && apt-get install -y ca-certificates

# Adding our root CA to make self-signed SSL cert valid.
ADD certs/ca.crt /usr/local/share/ca-certificates
RUN update-ca-certificates

ADD go-issue-25009 /usr/bin

ENTRYPOINT ["/usr/bin/go-issue-25009"]
