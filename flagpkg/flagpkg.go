// flagpkg package provides some additional flag functions. (InverseFlagVar)
package flagpkg

import (
	"flag"
	"fmt"
	"strconv"
)

// InverseBoolVar defines a flag that inverts a bool value.
//
// For example, "--no-foo" would set foo to false.
//
// Using --no-foo=false would set to true.
//
// Omitting flag does not change the value at all.
//
// If multiple flag.BoolVar and InverseBoolVar are used, the last one (on cmdline) wins.
func InverseBoolVar(p *bool, name string, value bool, usage string) {
	flag.CommandLine.Var(newBoolValue(value, p), name, usage)
}

// -- inversebool  Value
// mostly from https://go.dev/src/flag/flag.go
// except: we invert the value below, in Set
type inverseboolValue bool

func newBoolValue(val bool, p *bool) *inverseboolValue {
	*p = val
	return (*inverseboolValue)(p)
}

func (b *inverseboolValue) Set(s string) error {
	v, err := strconv.ParseBool(s)
	if err != nil {
		err = fmt.Errorf("invalid bool value: %v", err)
	}
	*b = inverseboolValue(!v) // invert value
	return err
}

func (b *inverseboolValue) Get() any { return bool(*b) }

func (b *inverseboolValue) String() string { return strconv.FormatBool(bool(*b)) }

func (b *inverseboolValue) IsBoolFlag() bool { return true }
