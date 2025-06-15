package config

import (
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

const (
	MulaURL         = "https://deshimula.com"
	OakURL          = "https://oakthu.com"
	Interval        = 1 * time.Minute
	StorageDir      = "storage"
	MulaStorageFile = "mula_sent_stories.json"
	OakStorageFile  = "oak_sent_stories.json"
)

type HTTPConfig struct {
	Headers map[string]string
	Client  *http.Client
}

func NewHTTPConfig() *HTTPConfig {
	headers := map[string]string{
		"User-Agent":                "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36",
		"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
		"Accept-Language":           "en-US,en;q=0.9",
		"Accept-Encoding":           "zstd, gzip, deflate, br",
		"Cache-Control":             "no-cache",
		"Connection":                "keep-alive",
		"Sec-Fetch-Dest":            "document",
		"Sec-Fetch-Mode":            "navigate",
		"Sec-Fetch-Site":            "none",
		"Sec-Fetch-User":            "?1",
		"Upgrade-Insecure-Requests": "1",
		"sec-ch-ua":                 `"Not.A/Brand";v="8", "Chromium";v="114", "Google Chrome";v="114"`,
		"sec-ch-ua-mobile":          "?0",
		"sec-ch-ua-platform":        `"Windows"`,
		"Pragma":                    "no-cache",
		"DNT":                       "1",
	}

	transport := &http.Transport{
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		MaxIdleConnsPerHost: 10,
		DisableCompression:  false,
		ForceAttemptHTTP2:   true,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	customTransport := &customTransport{
		Transport: transport,
	}

	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: customTransport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return http.ErrUseLastResponse
			}
			for key, values := range via[0].Header {
				req.Header[key] = values
			}
			return nil
		},
	}

	return &HTTPConfig{
		Headers: headers,
		Client:  client,
	}
}

var (
	zstdDecoderPool sync.Pool
)

func init() {
	zstdDecoderPool.New = func() interface{} {
		decoder, err := zstd.NewReader(nil)
		if err != nil {
			return nil
		}
		return decoder
	}
}

// customTransport wraps http.Transport to handle various compression methods
type customTransport struct {
	*http.Transport
}

func (t *customTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Add compression headers
	if req.Header.Get("Accept-Encoding") == "" {
		req.Header.Set("Accept-Encoding", "gzip, deflate, br, zstd")
	}

	resp, err := t.Transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	contentEncoding := resp.Header.Get("Content-Encoding")

	switch {
	case strings.Contains(contentEncoding, "zstd"):
		decoder := zstdDecoderPool.Get().(*zstd.Decoder)
		if decoder != nil {
			decoder.Reset(resp.Body)
			originalBody := resp.Body
			resp.Body = io.NopCloser(decoder)
			resp.Header.Del("Content-Encoding")
			resp.Header.Del("Content-Length")
			resp.ContentLength = -1
			resp.Uncompressed = true
			// Return decoder to pool when done
			resp.Body = struct {
				io.Reader
				io.Closer
			}{
				Reader: resp.Body,
				Closer: closerFunc(func() error {
					originalBody.Close()
					decoder.Reset(nil)
					zstdDecoderPool.Put(decoder)
					return nil
				}),
			}
		}
	case strings.Contains(contentEncoding, "br"):
		body := resp.Body
		resp.Body = io.NopCloser(brotli.NewReader(body))
		resp.Header.Del("Content-Encoding")
		resp.Header.Del("Content-Length")
		resp.ContentLength = -1
		resp.Uncompressed = true
	}

	return resp, nil
}

type closerFunc func() error

func (f closerFunc) Close() error { return f() }
