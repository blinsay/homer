package main

import (
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"

	"github.com/miekg/dns"
)

var (
	// args.
	endpoint string
	name     string

	// flags

	// use GET
	useGET = flag.Bool("get", false, "use a GET request to make a query instead of a POST request")
	// print raw http requests and responses
	dumpHTTP = flag.Bool("dump-http", false, "dumps http request/response headers")
	// the query type. parsed as a string, and then as an int if unrecognized.
	qtypeArg = flag.String("type", "A", "the `type` of record to query for. defaults to A")
	// the query class. defaults to IN, will never change?
	qclassArg = flag.String("class", "IN", "the `class` of record to query for. defaults to IN")
)

func init() {
	log.SetFlags(0)

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: %s [query flags] [endpoint] [names...]\n\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "%s makes a dns-over-https query to the given endpoint", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "available options:\n")
		flag.PrintDefaults()
	}
	flag.Parse()
}

func main() {
	args := flag.Args()
	if len(args) < 2 {
		log.Fatalln("endpoint and a query name are both required")
	}

	qclass, ok := qclassToInt(*qclassArg)
	if !ok {
		log.Fatalf("unrecognized query class: %s", *qclassArg)
	}
	qtype, ok := qtypeToSInt(*qtypeArg)
	if !ok {
		log.Fatalf("unrecognized query type: %s", *qtypeArg)
	}

	client := http.Client{}

	endpoint := args[0]
	for _, name := range args[1:] {
		// build the request and dump it before sending.
		var request *http.Request
		var err error
		if *useGET {
			request, err = NewGetRequest(endpoint, dns.Fqdn(name), qclass, qtype, true)
		} else {
			request, err = NewPostRequest(endpoint, dns.Fqdn(name), qclass, qtype, true)
		}
		if err != nil {
			log.Fatalln("error building request:", err)
		}
		request.Header.Set("User-Agent", "github.com/blinsay/homer")

		if *dumpHTTP {
			requestBs, err := httputil.DumpRequestOut(request, false)
			if err != nil {
				panic(err)
			}
			log.Println(string(requestBs))

			bodyBs, err := copyRequestBody(request)
			if err != nil {
				panic(err)
			}
			if len(bodyBs) > 0 {
				log.Println(hex.Dump(bodyBs))
			}
		}

		response, err := client.Do(request)
		if err != nil {
			log.Fatal(err)
		}

		// dump the response http request and body separately so that the response
		// can get hexdumped. this isn't toally necessary but makes it easier to
		// inspect the dns wire format.
		if *dumpHTTP {
			bs, err := httputil.DumpResponse(response, false)
			if err != nil {
				panic(err)
			}
			log.Println(string(bs))

			bodyBs, err := copyResponseBody(response)
			if err != nil {
				panic(err)
			}
			if len(bodyBs) > 0 {
				log.Println(hex.Dump(bodyBs))
			}
		}

		// parse the dns response if and only if we got a 200
		if response.StatusCode != 200 {
			log.Fatalln("error: unexpcted response code:", response.StatusCode)
		}
		dnsResponse, err := ReadDNSResponse(response)
		if err != nil {
			log.Fatal("error:", err)
		}
		fmt.Println(dnsResponse)
	}
}

func qclassToInt(qclass string) (uint16, bool) {
	qclass = strings.ToUpper(qclass)

	for id, name := range dns.ClassToString {
		if qclass == name {
			return id, true
		}
	}

	// TODO(benl): try parsing qclass as an int and returning that

	return 0, false
}

func qtypeToSInt(qtype string) (uint16, bool) {
	qtype = strings.ToUpper(qtype)

	for id, name := range dns.TypeToString {
		if qtype == name {
			return id, true
		}
	}

	// TODO(benl): try parsing qclass as an int and returning that

	return 0, false
}

func appendToFile(filename string, bs []byte) error {
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}

	n, err := f.Write(bs)
	if err == nil && n < len(bs) {
		err = io.ErrShortWrite
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

// try to copy a request body and replace it with a bytes.Buffer
func copyRequestBody(r *http.Request) ([]byte, error) {
	if r.Body == http.NoBody || r.Body == nil {
		return nil, nil
	}

	oldb := r.Body
	defer oldb.Close()

	bs, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	r.Body = ioutil.NopCloser(bytes.NewBuffer(bs))
	r.ContentLength = int64(len(bs))
	r.GetBody = func() (io.ReadCloser, error) {
		return ioutil.NopCloser(bytes.NewReader(bs)), nil
	}
	return bs, nil
}

// try to copy a response body and replace it with a bytes.Buffer
func copyResponseBody(r *http.Response) ([]byte, error) {
	if r.Body == http.NoBody || r.Body == nil {
		return nil, nil
	}

	oldb := r.Body
	defer oldb.Close()

	bs, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	r.Body = ioutil.NopCloser(bytes.NewBuffer(bs))

	return bs, nil
}

// hexdump formats bs sort of like xxd
func hexdump(bs []byte) (formatted []byte) {
	w := bytes.NewBuffer(formatted)

	for i := 0; i < len(bs); i += 16 {
		rowEnd := i + 16
		if rowEnd > len(bs) {
			rowEnd = len(bs)
		}
		row := bs[i:rowEnd]

		offset := fmt.Sprintf("0x%08x:", i/16)
		var cols []string
		var ascii []byte
		for j := 0; j < len(row); j += 2 {
			colEnd := j + 2
			if colEnd > len(row) {
				colEnd = len(row)
			}
			cols = append(cols, fmt.Sprintf("%x", row[j:colEnd]))
		}

		for _, b := range row {
			if 33 <= b && b <= 126 {
				ascii = append(ascii, b)
			} else {
				ascii = append(ascii, '.')
			}
		}

		fmt.Fprintf(w, "%s  %-039s  %s", offset, strings.Join(cols, " "), string(ascii))
	}
	return
}
