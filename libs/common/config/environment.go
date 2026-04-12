package config

import "fmt"

type Environment string

const (
	Development Environment = "development"
	Staging     Environment = "staging"
	Production  Environment = "production"
)

func (e Environment) IsValid() error {
	switch e {
	case Development, Staging, Production:
		return nil
	default:
		return fmt.Errorf("unknown environment %q: must be one of development, staging, production", e)
	}
}

func (e Environment) Is(other Environment) bool {
	return e == other
}

func (e Environment) String() string {
	return string(e)
}
