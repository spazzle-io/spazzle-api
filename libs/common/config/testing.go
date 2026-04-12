package config

// NewTestChainRegistry constructs a ChainRegistry for use in tests.
// It bypasses the environment-based lookup and sets the chain directly.
//
// Only for use in tests. Do not call from production code.
func NewTestChainRegistry(chain Chain) *ChainRegistry {
	return &ChainRegistry{
		current: chain,
		byID:    map[uint64]Chain{chain.ID: chain},
	}
}
