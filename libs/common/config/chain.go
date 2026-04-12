package config

import "fmt"

type Chain struct {
	ID   uint64
	Name string
	Slug string
}

type chainDef struct {
	Chain
	envs []Environment
}

type ChainRegistry struct {
	current Chain
	byID    map[uint64]Chain
}

// chainDefs is the single source of truth for all chains the system knows about and which environments they belong to.
// note that there can be only one chain per environment.
var chainDefs = []chainDef{
	{
		Chain: Chain{ID: 137, Name: "Polygon Mainnet", Slug: "polygon-mainnet"},
		envs:  []Environment{Production},
	},
	{
		Chain: Chain{ID: 11155111, Name: "Ethereum Sepolia", Slug: "ethereum-sepolia"},
		envs:  []Environment{Development, Staging},
	},
}

func newChainRegistry(env Environment) (*ChainRegistry, error) {
	var allowedChains []Chain
	byID := make(map[uint64]Chain, len(chainDefs))

	for _, def := range chainDefs {
		if _, dup := byID[def.ID]; dup {
			return nil, fmt.Errorf("duplicate chain ID %d", def.ID)
		}
		byID[def.ID] = def.Chain

		for _, e := range def.envs {
			if e == env {
				allowedChains = append(allowedChains, def.Chain)
			}
		}
	}

	switch len(allowedChains) {
	case 1:
		return &ChainRegistry{current: allowedChains[0], byID: byID}, nil
	case 0:
		return nil, fmt.Errorf("no chain configured for environment %q", env.String())
	default:
		return nil, fmt.Errorf("environment %q has %d chains configured. expected only one",
			env.String(), len(allowedChains))
	}
}

func (r *ChainRegistry) Current() Chain {
	return r.current
}

func (r *ChainRegistry) ByID(id uint64) (Chain, bool) {
	c, ok := r.byID[id]
	return c, ok
}

func (r *ChainRegistry) Validate(id uint64) (Chain, error) {
	if r.current.ID == id {
		return r.current, nil
	}

	if c, ok := r.byID[id]; ok {
		return Chain{}, fmt.Errorf("chain %s (id %d) is not permitted in this environment. expected %s (id %d)",
			c.Name, c.ID, r.current.Name, r.current.ID)
	}

	return Chain{}, fmt.Errorf("unknown chain ID %d", id)
}

func (r *ChainRegistry) All() []Chain {
	chains := make([]Chain, 0, len(r.byID))
	for _, c := range r.byID {
		chains = append(chains, c)
	}

	return chains
}
