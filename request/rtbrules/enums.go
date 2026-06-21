package rtbrules

import (
	"encoding/json"
	"strings"
)

type Aplicable int8

const (
	Exclude Aplicable = -1
	Any     Aplicable = 0
	Include Aplicable = 1
)

func (a Aplicable) String() string {
	switch a {
	case Exclude:
		return "exclude"
	case Any:
		return "any"
	case Include:
		return "include"
	}
	return ""
}

func (a Aplicable) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	return a.UnmarshalText([]byte(s))
}

func (a Aplicable) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.String())
}

func (a *Aplicable) UnmarshalText(text []byte) error {
	switch strings.ToLower(string(text)) {
	case "exclude", "exc", "exclude_all":
		*a = Exclude
	case "any", "*":
		*a = Any
	case "include", "inc", "include_all":
		*a = Include
	}
	return nil
}
