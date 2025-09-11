package cluster

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/k0sproject/k0sctl/internal/shell"
)

// Flags is a slice of strings with added functions to ease manipulating lists of command-line flags
type Flags []string

// Add adds a flag regardless if it exists already or not
func (f *Flags) Add(s string) {
	if ns, err := shell.Unquote(s); err == nil {
		s = ns
	}
	*f = append(*f, s)
}

// Add a flag with a value
func (f *Flags) AddWithValue(key, value string) {
	if nv, err := shell.Unquote(value); err == nil {
		value = nv
	}
	*f = append(*f, key+"="+value)
}

// AddUnlessExist adds a flag unless one with the same prefix exists
func (f *Flags) AddUnlessExist(s string) {
	if ns, err := shell.Unquote(s); err == nil {
		s = ns
	}
	if f.Include(s) {
		return
	}
	f.Add(s)
}

// AddOrReplace replaces a flag with the same prefix or adds a new one if one does not exist
func (f *Flags) AddOrReplace(s string) {
	if ns, err := shell.Unquote(s); err == nil {
		s = ns
	}
	idx := f.Index(s)
	if idx > -1 {
		(*f)[idx] = s
		return
	}
	f.Add(s)
}

// Include returns true if a flag with a matching prefix can be found
func (f Flags) Include(s string) bool {
	return f.Index(s) > -1
}

// Index returns an index to a flag with a matching prefix
func (f Flags) Index(s string) int {
	if ns, err := shell.Unquote(s); err == nil {
		s = ns
	}
	var flag string
	sepidx := strings.IndexAny(s, "= ")
	if sepidx < 0 {
		flag = s
	} else {
		flag = s[:sepidx]
	}
	for i, v := range f {
		if v == s || strings.HasPrefix(v, flag+"=") || strings.HasPrefix(v, flag+" ") {
			return i
		}
	}
	return -1
}

// Get returns the full flag with the possible value such as "--san=10.0.0.1" or "" when not found
func (f Flags) Get(s string) string {
	idx := f.Index(s)
	if idx < 0 {
		return ""
	}
	return f[idx]
}

// GetValue returns the value part of a flag such as "10.0.0.1" for a flag like "--san=10.0.0.1"
func (f Flags) GetValue(s string) string {
	fl := f.Get(s)
	if fl == "" {
		return ""
	}
	if nfl, err := shell.Unquote(fl); err == nil {
		fl = nfl
	}

	idx := strings.IndexAny(fl, "= ")
	if idx < 0 {
		return ""
	}

	val := fl[idx+1:]

	return val
}

// GetValue returns the boolean value part of a flag such as true for a flag like "--san"
// If the flag is not defined returns false. If the flag is defined without a value, returns true
// If no value is set, returns true
func (f Flags) GetBoolean(s string) (bool, error) {
	idx := f.Index(s)
	if idx < 0 {
		return false, nil
	}

	fl := f.GetValue(s)
	if fl == "" {
		return true, nil
	}

	return strconv.ParseBool(fl)
}

// Delete removes a matching flag from the list
func (f *Flags) Delete(s string) {
	idx := f.Index(s)
	if idx < 0 {
		return
	}
	*f = append((*f)[:idx], (*f)[idx+1:]...)
}

// Merge takes the flags from another Flags and adds them to this one unless this already has that flag set
func (f *Flags) Merge(b Flags) {
	for _, flag := range b {
		f.AddUnlessExist(flag)
	}
}

// MergeOverwrite takes the flags from another Flags and adds or replaces them into this one
func (f *Flags) MergeOverwrite(b Flags) {
	for _, flag := range b {
		f.AddOrReplace(flag)
	}
}

// MergeAdd takes the flags from another Flags and adds them into this one even if they exist
func (f *Flags) MergeAdd(b Flags) {
	for _, flag := range b {
		f.Add(flag)
	}
}

// Join creates a string separated by spaces
func (f *Flags) Join(cfg quoter) string {
	var parts []string
	f.Each(func(k, v string) {
		if v == "" && k != "" {
			parts = append(parts, quote(cfg, k))
		} else {
			parts = append(parts, fmt.Sprintf("%s=%s", k, quote(cfg, v)))
		}
	})
	return strings.Join(parts, " ")
}

// Each iterates over each flag and calls the function with the flag key and value as arguments
func (f Flags) Each(fn func(string, string)) {
	for _, flag := range f {
		sepidx := strings.IndexAny(flag, "= ")
		if sepidx < 0 {
			if flag == "" {
				continue
			}
			fn(flag, "")
		} else {
			key, value := flag[:sepidx], flag[sepidx+1:]
			if unq, err := shell.Unquote(value); err == nil {
				value = unq
			}
			fn(key, value)
		}
	}
}

// Map returns a map[string]string of the flags where the key is the flag and the value is the value
func (f Flags) Map() map[string]string {
	res := make(map[string]string)
	f.Each(func(k, v string) {
		res[k] = v
	})
	return res
}

// Equals compares the flags with another Flags and returns true if they have the same flags and values, ignoring order
func (f Flags) Equals(b Flags) bool {
	if len(f) != len(b) {
		return false
	}
	for _, flag := range f {
		if !b.Include(flag) {
			return false
		}
		ourValue := f.GetValue(flag)
		theirValue := b.GetValue(flag)
		if ourValue != theirValue {
			return false
		}
	}
	return true
}

// NewFlags shell-splits and parses a string and returns new Flags or an error if splitting fails
func NewFlags(s string) (Flags, error) {
	var flags Flags
	unq, err := shell.Unquote(s)
	if err != nil {
		return flags, fmt.Errorf("failed to unquote flags %q: %w", s, err)
	}
	parts, err := shell.Split(unq)
	if err != nil {
		return flags, fmt.Errorf("failed to split flags %q: %w", s, err)
	}
	for _, part := range parts {
		flags.Add(part)
	}
	return flags, nil
}
