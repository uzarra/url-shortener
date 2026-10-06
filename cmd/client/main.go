package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/go-resty/resty/v2"
	"github.com/uzarra/url-shortener/internal/api"
)

func main() {
	endpoint := "http://localhost:8080"
	contentType := "Content-Type"
	client := resty.New().
		SetRedirectPolicy(resty.NoRedirectPolicy())
	resp, err := client.R().
		SetHeader(contentType, "text/plain").
		SetBody(`http://yandex.ru`).
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
		URL: "http://yandex.ru",
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
		SetBody(gzipBody(`http://yandex.ru`)).
		Post(endpoint)
	if err != nil {
		panic(err)
	}
	printResponse("POST / gzip запрос text/plain", gzipPlainResponse)

	gzipNoTypeResponse, err := client.R().
		SetHeader(contentEncoding, "gzip").
		SetHeader(acceptEncoding, "gzip").
		SetBody(gzipBody(`http://yandex.ru`)).
		Post(endpoint)
	if err != nil {
		panic(err)
	}
	printResponse("POST / gzip запрос без Content-Type", gzipNoTypeResponse)
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
}
