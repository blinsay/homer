package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
	"time"

	"github.com/blinsay/homer/version"
	"golang.org/x/net/dns/dnsmessage"
	"golang.org/x/net/http2"
)

var (
	// dns-over-https options
	post              = flag.Bool("post", false, "use a POST request to make a query. slightly smaller, but less cache friendly")
	resolver          = flag.String("resolver", "", "the url of a dns-over-https resolver to use")
	bootstrapResolver = flag.String("bootstrap-resolver", "", "the ip address of a dns resolver to use to bootstrap the address of the dns-over-https resolver")
	noBootstrap       = flag.Bool("no-bootstrap", false, "don't do any name resolution for the dns-over-https resolver")

	// query options
	qtypeArg  = flag.String("type", "A", "the `type` of record to query for.")
	qclassArg = flag.String("class", "IN", "the `class` of record to query for.")

	// output options
	printVersion = flag.Bool("version", false, "print the version and exit")
	short        = flag.Bool("short", false, "provide a terse answer, like dig's +short")
	dumpHTTP     = flag.Bool("dump-http", false, "dumps http request/response headers")
)

var (
	stringToClass = map[string]dnsmessage.Class{
		"IN":  dnsmessage.ClassINET,
		"CS":  dnsmessage.ClassCSNET,
		"CH":  dnsmessage.ClassCHAOS,
		"HS":  dnsmessage.ClassHESIOD,
		"ANY": dnsmessage.ClassANY,
	}

	stringToType = map[string]dnsmessage.Type{
		"A":     dnsmessage.TypeA,
		"NS":    dnsmessage.TypeNS,
		"CNAME": dnsmessage.TypeCNAME,
		"SOA":   dnsmessage.TypeSOA,
		"PTR":   dnsmessage.TypePTR,
		"MX":    dnsmessage.TypeMX,
		"TXT":   dnsmessage.TypeTXT,
		"AAAA":  dnsmessage.TypeAAAA,
		"SRV":   dnsmessage.TypeSRV,
		"OPT":   dnsmessage.TypeOPT,
	}
)

const (
	schemeHTTPS           = "https"
	headerAccept          = "Accept"
	headerUserAgent       = "User-Agent"
	headerContentType     = "Content-Type"
	contentTypeDNSMessage = "application/dns-message"
	queryParameterDNS     = "dns"
)

var (
	userAgent = fmt.Sprintf("homer/%s", version.VERSION)
)

func init() {
	log.SetFlags(0)

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: %s [query flags] -resolver [url] [names...]\n\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "%s makes a dns-over-https query.\n\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "available options:\n")
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()

	if *printVersion {
		log.Printf("%s (%s)", version.VERSION, version.GITCOMMIT)
		return
	}

	if *resolver == "" {
		log.Fatalf("--resolver is required")
	}
	if *noBootstrap && (*bootstrapResolver != "") {
		log.Fatalf("--no-bootstrap and --bootstrap-resolver are incompatible")
	}

	qclass, ok := stringToClass[strings.ToUpper(*qclassArg)]
	if !ok {
		// FIXME: print supported classes
		log.Fatalf("unrecognized query class: %s", *qclassArg)
	}
	qtype, ok := stringToType[strings.ToUpper(*qtypeArg)]
	if !ok {
		// FIXME: print supported types
		log.Fatalf("unrecognized query type: %s", *qtypeArg)
	}

	// configure an http client to use for dns-over-https.
	//
	// without specifying any special dns bootstrap, the client should use a
	// transport that looks as close to http.DefaultTransport as possible.
	//
	// when a client disiables bootstrap dns, the Dialer's Resolver will always
	// return an error when Dial is called.
	//
	// when a client sets up a custom bootstrap dns server, the Dialer's Resovler
	// will always connect to the custom resolver, regardless of the address
	// passed to Dial.
	dialer := net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		DualStack: true,
	}
	if *noBootstrap {
		dialer.Resolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				return nil, fmt.Errorf("bootstrap dns is diabled")
			},
		}
	}

	if *bootstrapResolver != "" {
		if parsed := net.ParseIP(*bootstrapResolver); parsed == nil {
			log.Fatalf("boostrap-resolver must be an IP address")
		}

		bootstrapResolverAddress := net.JoinHostPort(*bootstrapResolver, "53")
		dialer.Resolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, network, bootstrapResolverAddress)
			},
		}
	}

	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	if err := http2.ConfigureTransport(transport); err != nil {
		panic(fmt.Sprintf("unable to setup http2: %s", err))
	}
	client := http.Client{Transport: transport}

	// per the RFC, application/dns-message requests should use a message id of
	// zero for cache friendliness.
	//
	// TODO(benl): a cli flag to disable recursive queries
	question := dnsmessage.Message{
		Header: dnsmessage.Header{
			ID:               0,
			RecursionDesired: true,
		},
	}

	for _, name := range flag.Args() {
		canonicalName := name
		if !strings.HasSuffix(canonicalName, ".") {
			canonicalName = canonicalName + "."
		}
		question.Questions = append(question.Questions, dnsmessage.Question{
			Class: qclass,
			Type:  qtype,
			Name:  dnsmessage.MustNewName(canonicalName),
		})
	}

	var err error
	var request *http.Request
	if *post {
		request, err = postRequest(*resolver, question)
	} else {
		request, err = getRequest(*resolver, question)
	}
	if err != nil {
		log.Fatalln("error building request:", err)
	}
	request.Header.Set(headerUserAgent, userAgent)

	if *dumpHTTP {
		reqDump, err := dumpRequest(request)
		if err != nil {
			panic(err)
		}
		log.Println(reqDump)
	}

	response, err := client.Do(request)
	if err != nil {
		log.Fatal(err)
	}

	if *dumpHTTP {
		respDump, err := dumpResponse(response)
		if err != nil {
			panic(err)
		}
		log.Println(respDump)
	}

	if response.StatusCode == 200 {
		msg, err := dnsResponse(response)
		if err != nil {
			log.Fatalf("error parsing response from server: %s", err)
		}

		for _, answer := range msg.Answers {
			if *short {
				log.Println(formatBody(answer.Body))
			} else {
				log.Println(formatHeader(answer.Header), formatBody(answer.Body))
			}
		}
	}
}

// pack a dns Message into a POST request. according to the RFC, POST requests
// should include Accept: application/dns-message, and should set their
// content-type appropriately
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
		return nil, fmt.Errorf("dns-over-https requires https: url scheme is %q", req.URL.Scheme)
	}

	req.Header.Set(headerContentType, contentTypeDNSMessage)
	req.Header.Set(headerAccept, contentTypeDNSMessage)

	return req, nil
}

// pack a dns Message into a GET request. according to the RFC, GET requests
// should include Accept: application/dns-message, and must base64 their request
// into a "dns" query parameter
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
		return nil, fmt.Errorf("dns-over-https requires https: url scheme is %q", req.URL.Scheme)
	}

	req.Header.Set(headerAccept, contentTypeDNSMessage)

	query := req.URL.Query()
	query.Set(queryParameterDNS, encodedMessage)
	req.URL.RawQuery = query.Encode()

	return req, nil
}

// parse a DNS response out of an http response body. returns an error if the
// content-type isn't application/dns-message or if there is no body
// to the response
func dnsResponse(response *http.Response) (*dnsmessage.Message, error) {
	if response.Body == nil {
		return nil, fmt.Errorf("nil body")
	}

	if contentType := response.Header.Get(headerContentType); strings.ToLower(contentType) != contentTypeDNSMessage {
		return nil, fmt.Errorf("unrecognized content type: %q", contentType)
	}

	defer response.Body.Close()
	bs, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	if len(bs) == 0 {
		return nil, fmt.Errorf("body was empty")
	}

	var msg dnsmessage.Message
	if err := msg.Unpack(bs); err != nil {
		return nil, err
	}
	return &msg, nil
}

// format a dns message header for output. looks a little bit like dig, with
// less whitepsace
func formatHeader(h dnsmessage.ResourceHeader) string {
	typ := h.Type.String()
	if strings.HasPrefix(typ, "Type") {
		typ = typ[4:]
	}
	return fmt.Sprintf("%s %d %s", h.Name, h.TTL, typ)
}

// format a dns resource body for output. mimics dig's output where appropriate
func formatBody(b dnsmessage.ResourceBody) string {
	switch rr := b.(type) {
	case *dnsmessage.AResource:
		return net.IP(rr.A[:]).String()
	case *dnsmessage.NSResource:
		return rr.NS.String()
	case *dnsmessage.CNAMEResource:
		return rr.CNAME.String()
	case *dnsmessage.SOAResource:
		return fmt.Sprintf("%s %s %d %d %d %d %d", rr.NS, rr.MBox, rr.Serial, rr.Refresh, rr.Retry, rr.Expire, rr.MinTTL)
	case *dnsmessage.PTRResource:
		return rr.PTR.String()
	case *dnsmessage.MXResource:
		return fmt.Sprintf("%d %s", rr.Pref, rr.MX)
	case *dnsmessage.TXTResource:
		return fmt.Sprintf("%q", strings.Join(rr.TXT, " "))
	case *dnsmessage.AAAAResource:
		return net.IP(rr.AAAA[:]).String()
	}

	return "(unknown)"
}

// dump an http request as a string, including the body if present. buffers
// the entire body into memory.handles replacing the body with a copy.
func dumpRequest(request *http.Request) (string, error) {
	requestBytes, err := httputil.DumpRequestOut(request, false)
	if err != nil {
		return "", err
	}
	headers := string(requestBytes)

	if request.Body == http.NoBody || request.Body == nil {
		return headers, nil
	}

	oldBody := request.Body
	defer oldBody.Close()

	bodyBytes, err := ioutil.ReadAll(request.Body)
	if err != nil {
		return "", err
	}

	request.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
	request.ContentLength = int64(len(bodyBytes))
	request.GetBody = func() (io.ReadCloser, error) {
		return ioutil.NopCloser(bytes.NewReader(bodyBytes)), nil
	}

	if len(bodyBytes) > 0 {
		return strings.Join([]string{headers, hex.Dump(bodyBytes)}, "\n"), nil
	}
	return headers, nil
}

// dump an http response as a string, including the body if present. buffers
// the entire body into memory. handles replacing the body with a copy.
func dumpResponse(response *http.Response) (string, error) {
	bs, err := httputil.DumpResponse(response, false)
	if err != nil {
		return "", err
	}
	headers := string(bs)

	oldBody := response.Body
	defer oldBody.Close()

	bodyBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	response.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))

	if len(bodyBytes) > 0 {
		return strings.Join([]string{headers, hex.Dump(bodyBytes)}, "\n"), nil
	}
	return headers, nil
}
