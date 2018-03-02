### D'OH

A toy DNS-over-HTTP client. Don't use this.

```
$ homer https://dns.google.com/experimental example.com
;; opcode: QUERY, status: NOERROR, id: 0
;; flags: qr rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0

;; QUESTION SECTION:
;example.com.	IN	 A

;; ANSWER SECTION:
example.com.	15736	IN	A	93.184.216.34
```

Run `homer --help` for usage info.  Build `homer` with a working Go toolchain
and `go install`.
