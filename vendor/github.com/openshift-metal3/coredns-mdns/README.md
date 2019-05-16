# mDNS

## Name

mDNS - CoreDNS plugin that reads mDNS records from the local network and responds
to queries based on those records.

## Description

Useful for providing mDNS records to non-mDNS-aware applications by making them
accessible through a standard DNS server.

## Syntax

~~~
mdns example.com [minimum SRV records]
~~~

## Examples

As a prerequisite to using this plugin, there must be systems on the local
network broadcasting mDNS records. Note that the .local domain will be
replaced with the configured domain. For example, `test.local` would become
`test.example.com` using the configuration below.

Specify the domain for the records.

~~~ corefile
example.com {
	mdns example.com
}
~~~

And test with `dig`:

~~~ txt
dig @localhost baremetal-test-extra-1.example.com

;; ANSWER SECTION:
baremetal-test-extra-1.example.com. 60 IN A   12.0.0.24
baremetal-test-extra-1.example.com. 60 IN AAAA fe80::f816:3eff:fe49:19b3
~~~

If `minimum SRV records` is specified in the configuration, the plugin will wait
until it has at least that many SRV records before responding with any of them.
`minimum SRV records` defaults to `3`.

~~~ corefile
example.com {
    mdns example.com 2
}
~~~

This would mean that at least two SRV records of a given type would need to be
present for any SRV records to be returned. If only one record is found, any
requests for that type of SRV record would receive no results.
