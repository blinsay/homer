package main

import (
	"bytes"
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

	"github.com/blinsay/homer/version"
	"golang.org/x/net/dns/dnsmessage"
)

var (
	// dns-over-https options
	post     = flag.Bool("post", false, "use a POST request to make a query. slightly smaller, but less cache friendly")
	resolver = flag.String("resolver", "", "the url of the dns-over-https resolver to use")

	// query options
	qtypeArg  = flag.String("type", "A", "the `type` of record to query for. defaults to A")
	qclassArg = flag.String("class", "IN", "the `class` of record to query for. defaults to IN")

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

	// TODO(benl): customize the bootstrap resolver for the dns-over-https server
	// TODO(benl): force http2?
	client := http.Client{}

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
			log.Fatal("error parsing dns-over-http response:", err)
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

func formatHeader(h dnsmessage.ResourceHeader) string {
	typ := h.Type.String()
	if strings.HasPrefix(typ, "Type") {
		typ = typ[4:]
	}
	return fmt.Sprintf("%s %d %s", h.Name, h.TTL, typ)
}

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
