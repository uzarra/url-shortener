package main

import (
	"errors"
	"fmt"

	"github.com/go-resty/resty/v2"
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
	fmt.Println("POST Статус-код ", resp.Status())
	newURL := resp.String()
	fmt.Printf("new url is = %s\n", newURL)

	getResp, err := client.R().
		SetHeader(contentType, "text/plain").
		Get(newURL)
	if err != nil && !errors.Is(err, resty.ErrAutoRedirectDisabled) {
		panic(err)
	}
	fmt.Println("GET Статус-код ", getResp.Status())
	location := getResp.Header().Get("Location")
	fmt.Printf("new location is = %s\n", location)

	shortenInBodyResponse, err := client.R().
		SetHeader(contentType, "application/json").
		SetBody(`{"url":"http://yandex.ru"}`).
		Post(endpoint + "/api/shorten")
	if err != nil {
		panic(err)
	}
	fmt.Println("POST Статус-код ", shortenInBodyResponse.Status())
	fmt.Printf("shortenInBodyResponse response is = %s", shortenInBodyResponse.String())
}
