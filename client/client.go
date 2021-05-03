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
	"path"
	"strconv"
	"strings"
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

	u.Path = path.Join(u.Path, "api")

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

func (c *Client) createReq(method, urlpath string, r io.Reader) (*http.Request, error) {
	u := *c.Url
	u.Path = path.Join(u.Path, urlpath)

	if c.Ctx != nil {
		return http.NewRequestWithContext(c.Ctx, method, u.String(), r)
	} else {
		return http.NewRequest(method, u.String(), r)
	}
}

func (c *Client) doRequest(request, urlpath string, r io.Reader) (*http.Response, error) {
	req, err := c.createReq(request, urlpath, r)
	if err != nil {
		return nil, err
	}
	resp, err := c.Http.Do(req)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode {
	case http.StatusOK:
		return resp, err
	default:
		return nil, fmt.Errorf("Received %d %s", resp.StatusCode,
			resp.Status)
	}
}

func (c *Client) GetRaw(urlpath string) ([]string, error) {
	resp, err := c.doRequest("GET", urlpath, nil)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	type data struct {
		Status string `json:"status"`
		Data   []struct {
			Date string
			Path string
			Id   float64
			Text string
		} `json:"data"`
	}

	var d data
	err = json.Unmarshal(body, &d)
	if err != nil {
		return nil, err
	}

	if d.Status != "success" {
		err = fmt.Errorf("%v", d.Data)
	}

	ret := make([]string, 0, len(d.Data))
	for i := range d.Data {
		s := d.Data[i].Text
		ret = append(ret, s)
	}
	return ret, err
}

func (c *Client) Get(urlpath string, values interface{}) error {
	js, err := c.GetRaw(urlpath)
	if err != nil {
		return err
	}

	whole := `[` + strings.Join(js, `,`) + `]`
	err = json.Unmarshal([]byte(whole), values)
	return err
}

func (c *Client) PutRaw(urlpath string, json []byte) error {
	buf := bytes.NewBuffer(json)
	_, err := c.doRequest("PUT", urlpath, buf)
	return err
}

func (c *Client) Put(urlpath string, data interface{}) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return c.PutRaw(urlpath, b)
}

func (c *Client) Delete(urlpath string) error {
	_, err := c.doRequest("DELETE", urlpath, nil)
	return err
}
