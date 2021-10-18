package resolver

import (
	"crypto/tls"

	rdns "github.com/folbricht/routedns"
)

func DotClient(dotServer string) *rdns.DoTClient {
	config := tls.Config{
		InsecureSkipVerify: true,
	}

	// Resolve the query
	r, _ := rdns.NewDoTClient("stress-dns", dotServer, rdns.DoTClientOptions{TLSConfig: &config})

	return r
}
