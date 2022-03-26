package verify

import (
	"crypto/tls"

	rdns "github.com/folbricht/routedns"
	"github.com/miekg/dns"
)

func DoTServer(dotServer string) bool {
	config := tls.Config{
		InsecureSkipVerify: true,
	}

	// Resolve the query
	r, err := rdns.NewDoTClient("test-dot", dotServer, rdns.DoTClientOptions{TLSConfig: &config})
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
