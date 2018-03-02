package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/miekg/dns"
)

const (
	qpCT  = "ct"
	qpDNS = "dns"

	headerAccept      = "Accept"
	headerContentType = "Content-Type"

	schemeHTTPS = "https"

	ContentTypeDNS_UDPWireFormat = "application/dns-udpwireformat"
)

// NewPostRequest creates a DNS-over-http POST request using application/dns-udpwireformat.
func NewPostRequest(url, zone string, qclass, qtype uint16, recursionDesired bool) (*http.Request, error) {
	// pack the DNS request to bytes and use it as the body of the POST request.
	// the stdlib should set content-length automatically.
	bs, err := packedQuestion(zone, qclass, qtype, recursionDesired)
	if err != nil {
		return nil, err
	}
	body := bytes.NewBuffer(bs)

	// build the http request with the packed body
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, err
	}
	if req.URL.Scheme != schemeHTTPS {
		return nil, fmt.Errorf("dns-over-https requrires https (url scheme is %q)", req.URL.Scheme)
	}

	// match the content type to the packed body set above
	req.Header.Set(headerContentType, ContentTypeDNS_UDPWireFormat)

	// clients MUST accept dns-udpwireformat responses according to the RFC
	req.Header.Set(headerAccept, ContentTypeDNS_UDPWireFormat)
	return req, nil
}

// NewGetRequest creates a DNS-over-http GET request using application/dns-udpwireformat.
func NewGetRequest(url, zone string, qclass, qtype uint16, recursionDesired bool) (*http.Request, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if req.URL.Scheme != schemeHTTPS {
		return nil, fmt.Errorf("dns-over-https requrires https (url scheme is %q)", req.URL.Scheme)
	}

	// clients MUST accept dns-udpwireformat responses
	req.Header.Set(headerAccept, ContentTypeDNS_UDPWireFormat)

	// a blank `ct` parameter indicates that the dns parameter is base64url
	// encoded application/dns-udpwireformat data
	query := req.URL.Query()
	query.Set(qpCT, "")

	// use base64url to pack the raw message. skips using dns.SetQuestion to
	// support other clasess of query and to make sure that the ID is set to 0
	// to satisfy the RFC.
	bs, err := packedQuestion(zone, qclass, qtype, recursionDesired)
	if err != nil {
		return nil, err
	}
	query.Set(qpDNS, base64.RawURLEncoding.EncodeToString(bs))
	req.URL.RawQuery = query.Encode()

	return req, nil
}

func packedQuestion(name string, qclass, qtype uint16, recursionDesired bool) ([]byte, error) {
	m := new(dns.Msg)
	m.Question = []dns.Question{dns.Question{
		Name:   name,
		Qclass: qclass,
		Qtype:  qtype,
	}}
	m.RecursionDesired = recursionDesired
	return m.Pack()
}

// ParseDNSResponse parses the body of a DNS-over-http response. The response
// Body is fully consumed and closed by this func.
func ReadDNSResponse(response *http.Response) (*dns.Msg, error) {
	if response.Body == nil {
		return nil, fmt.Errorf("nil response body")
	}

	if contentType := response.Header.Get(headerContentType); ContentTypeDNS_UDPWireFormat != strings.ToLower(contentType) {
		return nil, fmt.Errorf("unrecognized content type: %s", contentType)
	}

	defer response.Body.Close()

	bs, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	m := new(dns.Msg)
	if err := m.Unpack(bs); err != nil {
		return nil, err
	}
	return m, nil
}
