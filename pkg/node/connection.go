package node

import "strings"

// Connection error substrings that indicate the SSH/exec session was lost
// and a reconnect may allow the next attempt to succeed.
var connectionErrorSubstrs = []string{
	"connection reset by peer",
	"connection refused",
	"broken pipe",
	"ssh session wait",
	"ssh new session",
	"i/o timeout",
	"use of closed network connection",
}

// IsConnectionError reports whether err (or any error in its chain) indicates
// a lost SSH or transport connection, so callers can disconnect and reconnect
// before retrying.
func IsConnectionError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	for _, sub := range connectionErrorSubstrs {
		if strings.Contains(msg, sub) {
			return true
		}
	}
	return false
}
