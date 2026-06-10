package cluster

type quoter interface {
	ShellQuote(string) string
}

func quote(q quoter, value string) string {
	return q.ShellQuote(value)
}
