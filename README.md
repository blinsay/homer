A toy [dns-over-https (doh) client](https://github.com/curl/curl/wiki/DNS-over-HTTPS).

![image](https://user-images.githubusercontent.com/555011/36924998-d8d90354-1e3e-11e8-9e8d-9141cc375b95.png)

```
$ homer --resolver https://dns.google.com/experimental blinsay.com
blinsay.com. 1190 A 185.199.110.153
blinsay.com. 1190 A 185.199.109.153
blinsay.com. 1190 A 185.199.111.153
blinsay.com. 1190 A 185.199.108.153
```

Get it from the [releases](https://github.com/blinsay/homer/releases) page.
Run `homer --help` for usage info.

The `curl` GitHub wiki has a list of
[public resolvers](https://github.com/curl/curl/wiki/DNS-over-HTTPS#publicly-available-servers)
for querying.

`homer` can dump the full dns-over-https request/response for you to examine.

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


### building

Build `homer` with a working Go toolchain and `go get github.com/blinsay/homer`

