package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"time"
)

// Upstream represents an upstream host.
type Upstream struct {
	Dial string
}

// ReverseProxy handles proxying requests to upstreams.
type ReverseProxy struct {
	Upstreams []*Upstream
	Transport http.RoundTripper
}

func (hp *ReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	if len(hp.Upstreams) == 0 {
		return errors.New("no upstreams available")
	}
	upstream := hp.Upstreams[0]

	upstreamURL, err := url.Parse("http://" + upstream.Dial)
	if err != nil {
		return err
	}

	outreq := r.Clone(r.Context())
	outreq.URL.Scheme = upstreamURL.Scheme
	outreq.URL.Host = upstreamURL.Host
	outreq.RequestURI = ""

	transport := hp.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	res, err := transport.RoundTrip(outreq)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(r.Context().Err(), context.Canceled) {
			return r.Context().Err()
		}
		return err
	}

	if res.Body != nil {
		reqCtx := r.Context()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			select {
			case <-ctx.Done():
			case <-reqCtx.Done():
				res.Body.Close()
			}
		}()
		defer res.Body.Close()
	}

	for k, vv := range res.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(res.StatusCode)

	if res.Body != nil {
		_, err = io.Copy(w, res.Body)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(r.Context().Err(), context.Canceled) {
				return r.Context().Err()
			}
			return err
		}
	}

	return nil
}

func main() {
	fmt.Println("Hello, Bounty Hunter!")
}
