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

Sometimes you don't know the IP address of the resolver you want to use and
need to look it up using... DNS. Since dns-over-https is about privacy and
security, `homer` lets you specify a DNS resolver you trust to do that initial
lookup.

```
$ homer --bootstrap-resolver 1.1.1.1 --resolver https://dns.google.com/experimental github.com
github.com. 57 A 192.30.253.112
```

### building and running

Download `homer` from the [releases
page](https://github.com/blinsay/homer/releases) on github.

Build `homer` with a working Go toolchain and `go get github.com/blinsay/homer`

