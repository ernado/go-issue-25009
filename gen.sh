#!/bin/bash
openssl req -x509 -nodes -subj '/CN=nginx' -days 1000 -newkey rsa:4096 -sha256 -keyout nginx.key -out nginx.crt
