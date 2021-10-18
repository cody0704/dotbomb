package verify

import (
	"crypto/tls"

	rdns "github.com/folbricht/routedns"
	"github.com/miekg/dns"
)

func VerifyDoTServer(dotServer string) bool {
	config := tls.Config{
		InsecureSkipVerify: true,
	}

	// Resolve the query
	r, _ := rdns.NewDoTClient("test-dot", dotServer, rdns.DoTClientOptions{TLSConfig: &config})

	// Build a query
	q := new(dns.Msg)
	q.SetQuestion("www.google.com.", dns.TypeA)

	_, err := r.Resolve(q, rdns.ClientInfo{})
	if err != nil {
		return false
	}

	return true
}
