package client

import (
	"net"
	"net/http"
	"time"
)

type Client struct {
	Http *http.Client
}

func NewClient(url string) (*Client, error) {
	tr := &http.Transport{
		Dial: (&net.Dialer{
			Timeout: time.Second * 5,
		}).Dial,
		TLSHandshakeTimeout: time.Second * 5,
	}

	return &Client{
		Http: &http.Client{
			Timeout:   time.Second * 10,
			Transport: tr,
		},
	}, nil
}

func (c *Client) Get(path string) (interface{}, error) {
	return nil, nil
}

func (c *Client) Put(path string, data interface{}) error {
	return nil
}
