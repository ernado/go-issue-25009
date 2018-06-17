#!/bin/bash
openssl genrsa -out ca.key 4096
openssl req -new -x509 -key ca.key -out ca.crt
openssl x509 -req -in nginx.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out nginx.crt
cat nginx.crt ca.crt > nginx.bundle.crt
