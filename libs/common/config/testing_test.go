package config

import "testing"

func TestNewTestChainRegistry_CurrentReturnsProvidedChain(t *testing.T) {
	chain := Chain{ID: 11155111, Name: "Sepolia"}
	r := NewTestChainRegistry(chain)

	got := r.Current()
	if got != chain {
		t.Errorf("expected %+v, got %+v", chain, got)
	}
}

func TestNewTestChainRegistry_ByID_FindsChain(t *testing.T) {
	chain := Chain{ID: 11155111, Name: "Sepolia"}
	r := NewTestChainRegistry(chain)

	got, ok := r.ByID(11155111)
	if !ok {
		t.Fatal("expected ByID to find the chain")
	}

	if got != chain {
		t.Errorf("expected %+v, got %+v", chain, got)
	}
}

func TestNewTestChainRegistry_ValidatePassesForProvidedChain(t *testing.T) {
	chain := Chain{ID: 11155111, Name: "Sepolia"}
	r := NewTestChainRegistry(chain)

	got, err := r.Validate(11155111)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != chain {
		t.Errorf("expected %+v, got %+v", chain, got)
	}
}

func TestNewTestChainRegistry_ValidateFailsForOtherChain(t *testing.T) {
	chain := Chain{ID: 11155111, Name: "Sepolia"}
	r := NewTestChainRegistry(chain)

	_, err := r.Validate(1)
	if err == nil {
		t.Fatal("expected error for chain not in registry, got nil")
	}
}
