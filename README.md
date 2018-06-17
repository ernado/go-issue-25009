# golang/go#25009

https://github.com/golang/go/issues/25009

```bash
$ git clone https://github.com/ernado/go-issue-25009.git
$ cd go-issue-25009
$ ./test.sh

# Possible environmental variables:
#
# CONCURRENT (bool) - use one http.Client for all goroutines (default 1)
# BODY (bool) - set req.GetBody explicitly (default 0)
# JOBS (int) - number of concurrent clients (default 6)
# REQUESTS (int) - number of request to do (default 100)
# HTTP2_TRANSPORT (bool) - use http2.Transport (default false)
# TLS_SKIP_VERIFY (bool) - InsecureSkipVerify  (default false)
# GODEBUG - debugging variables within the runtime for go

# Example:
$ REQUESTS=10 CONCURRENT=false ./test.sh
```