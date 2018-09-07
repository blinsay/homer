package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"golang.org/x/net/dns/dnsmessage"
)

const (
	schemeHTTPS           = "https"
	headerAccept          = "Accept"
	headerContentType     = "Content-Type"
	contentTypeDNSMessage = "application/dns-message"
)

func postRequest(resolverURL string, msg dnsmessage.Message) (*http.Request, error) {
	bs, err := msg.Pack()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, resolverURL, bytes.NewBuffer(bs))
	if err != nil {
		return nil, err
	}
	if req.URL.Scheme != schemeHTTPS {
		return nil, fmt.Errorf("dns-over-http requires https: url scheme is %q", req.URL.Scheme)
	}

	req.Header.Set(headerContentType, contentTypeDNSMessage)
	req.Header.Set(headerAccept, contentTypeDNSMessage)

	return req, nil
}

func getRequest(resolverURL string, msg dnsmessage.Message) (*http.Request, error) {
	bs, err := msg.Pack()
	if err != nil {
		return nil, err
	}
	encodedMessage := base64.RawURLEncoding.EncodeToString(bs)

	req, err := http.NewRequest(http.MethodGet, resolverURL, nil)
	if err != nil {
		return nil, err
	}
	if req.URL.Scheme != schemeHTTPS {
		return nil, fmt.Errorf("dns-over-http requires https: url scheme is %q", req.URL.Scheme)
	}

	req.Header.Set(headerAccept, contentTypeDNSMessage)

	query := req.URL.Query()
	query.Set("dns", encodedMessage)
	req.URL.RawQuery = query.Encode()

	return req, nil
}

func dnsResponse(response *http.Response) (*dnsmessage.Message, error) {
	if response.Body == nil {
		return nil, fmt.Errorf("nil body")
	}

	if contentType := response.Header.Get(headerContentType); strings.ToLower(contentType) != contentTypeDNSMessage {
		return nil, fmt.Errorf("dns-over-http: unregognized content type: %q", contentType)
	}

	defer response.Body.Close()
	bs, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	var msg dnsmessage.Message
	if err := msg.Unpack(bs); err != nil {
		return nil, err
	}
	return &msg, nil
}
