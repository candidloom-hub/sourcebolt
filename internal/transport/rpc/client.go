// Package rpc reaches NotebookLM through its internal web RPC endpoint.
package rpc

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

// NewHTTPClient returns the one shared client: HTTP/2, kept-alive
// connections and TLS session resumption, so only the first call pays for
// DNS, TCP and TLS. Accept-Encoding is left to net/http (it decompresses
// gzip transparently only when it set the header itself). There is no
// ResponseHeaderTimeout: unary calls bound their header wait per request
// (Options.HeaderTimeout) and the chat stream by StreamFirstChunk.
func NewHTTPClient() *http.Client {
	tr := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          32,
		MaxIdleConnsPerHost:   16,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
		TLSClientConfig:       &tls.Config{ClientSessionCache: tls.NewLRUClientSessionCache(64), MinVersion: tls.VersionTLS12},
	}
	return &http.Client{Transport: tr}
}
