package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"time"

	"github.com/kopoli/appkit"
)

type Client struct {
	Http *http.Client
	Url  *url.URL
	Ctx  context.Context
}

func NewClient(URL string, opts appkit.Options) (*Client, error) {
	parseTimeout := func(name string, def int) time.Duration {
		v := strconv.Itoa(def)
		val := opts.Get(name, v)

		vint, err := strconv.Atoi(val)
		if err != nil {
			vint = def
		}
		return time.Second * time.Duration(vint)
	}

	u, err := url.Parse(URL)
	if err != nil {
		return nil, err
	}

	u.Path = filepath.Join(u.Path, "api")

	tr := &http.Transport{
		Dial: (&net.Dialer{
			Timeout: parseTimeout("timeout-dial", 5),
		}).Dial,
		TLSHandshakeTimeout: parseTimeout("timeout-tls-handshake", 5),
	}

	return &Client{
		Http: &http.Client{
			Timeout:   parseTimeout("timeout-http-client", 10),
			Transport: tr,
		},
		Url: u,
		Ctx: nil,
	}, nil
}

func (c *Client) createReq(method, path string, r io.Reader) (*http.Request, error) {
	u := *c.Url
	u.Path = filepath.Join(u.Path, path)

	fmt.Println("Would send", method, "to URL", u.String())

	if c.Ctx != nil {
		return http.NewRequestWithContext(c.Ctx, method, u.String(), r)
	} else {
		return http.NewRequest(method, u.String(), r)
	}
}

func (c *Client) doRequest(request, path string, r io.Reader) (*http.Response, error) {
	req, err := c.createReq(request, path, r)
	if err != nil {
		return nil, err
	}
	resp, err := c.Http.Do(req)
	return resp, err
}

func (c *Client) Get(path string) (interface{}, error) {
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	type data struct {
		Status string      `json:"status"`
		Data   interface{} `json:"data"`
	}

	var d data
	err = json.Unmarshal(body, &d)
	if err != nil {
		return nil, err
	}

	fmt.Println("GET data:", d)

	if d.Status != "success" {
		err = fmt.Errorf("%v", d.Data)
	}

	return d.Data, err
}

func (c *Client) Put(path string, data interface{}) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	buf := bytes.NewBuffer(b)

	_, err = c.doRequest("PUT", path, buf)
	return err
}
