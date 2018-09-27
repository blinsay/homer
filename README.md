A toy [dns-over-https (doh)
client](https://github.com/curl/curl/wiki/DNS-over-HTTPS).

![image](https://user-images.githubusercontent.com/555011/36924998-d8d90354-1e3e-11e8-9e8d-9141cc375b95.png)

### huh?

dns-over-https is an [experimental
protocol](https://tools.ietf.org/html/draft-ietf-doh-dns-over-https) for making
DNS queries over https.  Even though the protocol is experimental, the `curl`
GitHub wiki has a list of [public
resolvers](https://github.com/curl/curl/wiki/DNS-over-HTTPS#publicly-available-servers)
that already support it.

`homer` is a CLI client that makes DNS queries over https.

```
$ homer --resolver https://1.1.1.1/dns-query blinsay.com
blinsay.com. 1190 A 185.199.110.153
blinsay.com. 1190 A 185.199.109.153
blinsay.com. 1190 A 185.199.111.153
blinsay.com. 1190 A 185.199.108.153
```

To see what's going on when you make a dns-over-https query, use the
`--dump-http` option.

```
$ homer --dump-http --resolver https://dns.google.com/experimental blinsay.com
GET /experimental?dns=AAABAAABAAAAAAAAB2JsaW5zYXkDY29tAAABAAE HTTP/1.1
Host: dns.google.com
User-Agent: homer/v0.0.1
Accept: application/dns-message
Accept-Encoding: gzip

HTTP/2.0 200 OK
Content-Length: 93
Content-Type: application/dns-message
Date: Sat, 08 Sep 2018 02:08:55 GMT

00000000  00 00 81 80 00 01 00 04  00 00 00 00 07 62 6c 69  |.............bli|
00000010  6e 73 61 79 03 63 6f 6d  00 00 01 00 01 c0 0c 00  |nsay.com........|
00000020  01 00 01 00 00 04 af 00  04 b9 c7 6e 99 c0 0c 00  |...........n....|
00000030  01 00 01 00 00 04 af 00  04 b9 c7 6f 99 c0 0c 00  |...........o....|
00000040  01 00 01 00 00 04 af 00  04 b9 c7 6c 99 c0 0c 00  |...........l....|
00000050  01 00 01 00 00 04 af 00  04 b9 c7 6d 99           |...........m.|

blinsay.com. 1199 A 185.199.110.153
blinsay.com. 1199 A 185.199.111.153
blinsay.com. 1199 A 185.199.108.153
blinsay.com. 1199 A 185.199.109.153
```

Sometimes you don't know the IP address of the resolver you want to use and
need to look it up using... DNS. Since dns-over-https is about privacy and
security, `homer` lets you specify a DNS resolver you trust to do that initial
lookup, rather than delegating to your operating system.

```
$ ./homer --bootstrap-resolver 1.1.1.1 --resolver https://dns.google.com/experimental github.com
github.com. 59 A 192.30.253.113
github.com. 59 A 192.30.253.112
```

Using the `--no-bootstrap` option, you can opt out of the process entirely,
and make sure you're never using that untrustworthy operating system resolver.


### building and running

Download `homer` from the [releases
page](https://github.com/blinsay/homer/releases) on github.

Build `homer` with a working Go toolchain and `go get github.com/blinsay/homer`

