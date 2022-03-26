package verify

import (
	rdns "github.com/folbricht/routedns"
	"github.com/miekg/dns"
)

func DNSServer(dnsServer string) bool {
	// Resolve the query
	r, err := rdns.NewDNSClient("test-dns", dnsServer, "udp", rdns.DNSClientOptions{})
	if err != nil {
		return false
	}

	// Build a query
	q := new(dns.Msg)
	q.SetQuestion("www.google.com.", dns.TypeA)

	if _, err = r.Resolve(q, rdns.ClientInfo{}); err != nil {
		return false
	}

	return true
}
