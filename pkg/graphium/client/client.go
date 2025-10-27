package client

import (
	"fmt"
	"net/http"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func New(baseURL string) (*Client, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("baseURL is required")
	}

	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}, nil
}

type Query struct {
	Status     string
	Host       string
	Datacenter string
	Limit      int
}
