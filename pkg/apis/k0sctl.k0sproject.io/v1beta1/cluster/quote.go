package cluster

import "github.com/k0sproject/k0sctl/configurer"

type quoter interface {
	Quote(string) string
}

var defaultQuoter quoter = &configurer.Linux{}

func quote(q quoter, value string) string {
	if q == nil {
		return defaultQuoter.Quote(value)
	}
	return q.Quote(value)
}
