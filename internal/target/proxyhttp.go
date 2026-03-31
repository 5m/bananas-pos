package target

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"bananas-pos/internal/job"
)

type ProxyHTTP struct {
	targetURL string
	target    *url.URL
	client    *http.Client
	proxy     *httputil.ReverseProxy
}

const proxyHTTPTimeout = 10 * time.Second

func NewProxyHTTP(targetURL string) (*ProxyHTTP, error) {
	target, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}

	reverseProxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := reverseProxy.Director
	reverseProxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = target.Host
	}
	reverseProxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
		log.Printf("proxy error for %s %s: %v", req.Method, req.URL.String(), err)
		http.Error(rw, "bad gateway", http.StatusBadGateway)
	}

	return &ProxyHTTP{
		targetURL: targetURL,
		target:    target,
		client:    &http.Client{Timeout: proxyHTTPTimeout},
		proxy:     reverseProxy,
	}, nil
}

func (p *ProxyHTTP) Name() string {
	return "http-proxy"
}

func (p *ProxyHTTP) Send(ctx context.Context, printJob job.PrintJob) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.target.String(), bytes.NewReader(printJob.Raw))
	if err != nil {
		return err
	}

	if printJob.ContentType != "" {
		req.Header.Set("Content-Type", printJob.ContentType)
	} else {
		req.Header.Set("Content-Type", "application/zpl")
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &httpError{statusCode: resp.StatusCode, status: resp.Status}
	}

	return nil
}

func (p *ProxyHTTP) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, p.target.String(), nil)
	if err != nil {
		return err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("upstream health check failed: %s", resp.Status)
	}
	return nil
}

func (p *ProxyHTTP) ServeHTTPProxy(rw http.ResponseWriter, req *http.Request) bool {
	p.proxy.ServeHTTP(rw, req)
	return true
}

type httpError struct {
	statusCode int
	status     string
}

func (e *httpError) Error() string {
	return e.status
}
