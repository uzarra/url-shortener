package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/uzarra/url-shortener/internal/api"
)

func main() {
	endpoint := "http://localhost:8080"
	contentType := "Content-Type"
	base := fmt.Sprintf("http://yandex.ru/client/%d", time.Now().UnixNano())
	client := resty.New().
		SetRedirectPolicy(resty.NoRedirectPolicy())
	resp, err := client.R().
		SetHeader(contentType, "text/plain").
		SetBody(base + "/plain").
		Post(endpoint)
	if err != nil {
		panic(err)
	}
	printResponse("POST /", resp)

	getResp, err := client.R().
		SetHeader(contentType, "text/plain").
		Get(resp.String())
	if err != nil && !errors.Is(err, resty.ErrAutoRedirectDisabled) {
		panic(err)
	}
	printResponse("GET /{id}", getResp)

	shortenRequest := api.ShortenRequest{
		URL: base + "/json",
	}
	shortenRequestBytes, err := json.Marshal(shortenRequest)
	if err != nil {
		panic(err)
	}
	shortenInBodyResponse, err := client.R().
		SetHeader(contentType, "application/json").
		SetBody(shortenRequestBytes).
		Post(endpoint + "/api/shorten")
	if err != nil {
		panic(err)
	}
	printResponse("POST /api/shorten обычный запрос", shortenInBodyResponse)

	gzipResponse, err := client.R().
		SetHeader(contentType, "application/json").
		SetHeader(contentEncoding, "gzip").
		SetHeader(acceptEncoding, "gzip").
		SetBody(gzipBody(string(shortenRequestBytes))).
		Post(endpoint + "/api/shorten")
	if err != nil {
		panic(err)
	}
	printResponse("POST /api/shorten gzip запрос + gzip ответ", gzipResponse)

	acceptGzipResponse, err := client.R().
		SetHeader(contentType, "application/json").
		SetHeader(acceptEncoding, "gzip").
		SetBody(shortenRequestBytes).
		Post(endpoint + "/api/shorten")
	if err != nil {
		panic(err)
	}
	printResponse("POST /api/shorten обычный запрос + gzip ответ", acceptGzipResponse)

	gzipPlainResponse, err := client.R().
		SetHeader(contentType, "text/plain").
		SetHeader(contentEncoding, "gzip").
		SetHeader(acceptEncoding, "gzip").
		SetBody(gzipBody(base + "/gzip-plain")).
		Post(endpoint)
	if err != nil {
		panic(err)
	}
	printResponse("POST / gzip запрос text/plain", gzipPlainResponse)

	gzipNoTypeResponse, err := client.R().
		SetHeader(contentEncoding, "gzip").
		SetHeader(acceptEncoding, "gzip").
		SetBody(gzipBody(base + "/gzip-notype")).
		Post(endpoint)
	if err != nil {
		panic(err)
	}
	printResponse("POST / gzip запрос без Content-Type", gzipNoTypeResponse)

	userURLsResponse, err := client.R().
		Get(endpoint + "/api/user/urls")
	if err != nil {
		panic(err)
	}
	printResponse("GET /api/user/urls тот же клиент", userURLsResponse)

	newClientResponse, err := resty.New().R().
		Get(endpoint + "/api/user/urls")
	if err != nil {
		panic(err)
	}
	printResponse("GET /api/user/urls новый клиент", newClientResponse)
}

const (
	contentEncoding = "Content-Encoding"
	acceptEncoding  = "Accept-Encoding"
)

func gzipBody(s string) []byte {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write([]byte(s)); err != nil {
		panic(err)
	}
	if err := zw.Close(); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func printResponse(name string, resp *resty.Response) {
	fmt.Printf("%s: Статус-код %s\n", name, resp.Status())
	fmt.Printf("  response headers = %s\n", resp.Header())
	fmt.Printf("  response is = %s\n", resp.String())
	fmt.Printf("  response cookie = %s\n", resp.Cookies())
	fmt.Printf("  request cookie = %s\n", resp.Request.RawRequest.Cookies())
}
