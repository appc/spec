package types

import (
	"encoding/json"
	"fmt"
	"sort"
)

var ValidOSArch = map[string][]string{
	"linux":   {"amd64", "i386"},
	"freebsd": {"amd64", "i386", "arm"},
	"darwin":  {"x86_64", "i386"},
}

type Labels []Label

type labels Labels

type Label struct {
	Name  ACName `json:"name"`
	Value string `json:"value"`
}

func (l Labels) assertValid() error {
	seen := map[ACName]string{}
	for _, lbl := range l {
		if lbl.Name == "name" {
			return fmt.Errorf(`invalid label name: "name"`)
		}
		_, ok := seen[lbl.Name]
		if ok {
			return fmt.Errorf(`duplicate labels of name %q`, lbl.Name)
		}
		seen[lbl.Name] = lbl.Value
	}
	if os, ok := seen["os"]; ok {
		if validArchs, ok := ValidOSArch[os]; !ok {
			// Not a whitelisted OS. TODO: how to warn rather than fail?
			validOses := make([]string, 0, len(ValidOSArch))
			for validOs := range ValidOSArch {
				validOses = append(validOses, validOs)
			}
			sort.Strings(validOses)
			return fmt.Errorf(`bad os %#v (must be one of: %v)`, os, validOses)
		} else {
			// Whitelisted OS. We check arch here, as arch makes sense only
			// when os is defined.
			if arch, ok := seen["arch"]; ok {
				found := false
				for _, validArch := range validArchs {
					if arch == validArch {
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf(`bad arch %#v for %v (must be one of: %v)`, arch, os, validArchs)
				}
			}
		}
	}
	return nil
}

func (l Labels) MarshalJSON() ([]byte, error) {
	if err := l.assertValid(); err != nil {
		return nil, err
	}
	return json.Marshal(labels(l))
}

func (l *Labels) UnmarshalJSON(data []byte) error {
	var jl labels
	if err := json.Unmarshal(data, &jl); err != nil {
		return err
	}
	nl := Labels(jl)
	if err := nl.assertValid(); err != nil {
		return err
	}
	*l = nl
	return nil
}

// Get retrieves the value of the label by the given name from Labels, if it exists
func (l Labels) Get(name string) (val string, ok bool) {
	for _, lbl := range l {
		if lbl.Name.String() == name {
			return lbl.Value, true
		}
	}
	return "", false
}
