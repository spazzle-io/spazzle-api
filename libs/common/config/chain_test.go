package config

import "testing"

func withChainDefinitions(defs []chainDef, fn func()) {
	orig := chainDefs
	chainDefs = defs
	defer func() {
		chainDefs = orig
	}()
	fn()
}

func TestNewChainRegistry(t *testing.T) {
	t.Run("development", func(t *testing.T) {
		r, err := newChainRegistry(Development)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if r.current.ID != 11155111 {
			t.Errorf("unexpected chain ID, got %d", r.current.ID)
		}
		if r.current.Name != "Ethereum Sepolia" {
			t.Errorf("unexpected name, got %q", r.current.Name)
		}
		if r.current.Slug != "ethereum-sepolia" {
			t.Errorf("unexpected slug, got %q", r.current.Slug)
		}
	})

	t.Run("staging", func(t *testing.T) {
		r, err := newChainRegistry(Staging)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if r.current.ID != 11155111 {
			t.Errorf("unexpected chain ID, got %d", r.current.ID)
		}
		if r.current.Name != "Ethereum Sepolia" {
			t.Errorf("unexpected name, got %q", r.current.Name)
		}
		if r.current.Slug != "ethereum-sepolia" {
			t.Errorf("unexpected slug, got %q", r.current.Slug)
		}
	})

	t.Run("production", func(t *testing.T) {
		r, err := newChainRegistry(Production)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if r.current.ID != 137 {
			t.Errorf("unexpected chain ID, got %d", r.current.ID)
		}
		if r.current.Name != "Polygon Mainnet" {
			t.Errorf("unexpected name, got %q", r.current.Name)
		}
		if r.current.Slug != "polygon-mainnet" {
			t.Errorf("unexpected slug, got %q", r.current.Slug)
		}
	})
}

func TestNewChainRegistry_UnknownEnvironment(t *testing.T) {
	_, err := newChainRegistry("testnet")
	if err == nil {
		t.Fatal("expected error for unknown environment, got nil")
	}
}

func TestNewChainRegistry_NoChainForEnv(t *testing.T) {
	withChainDefinitions([]chainDef{
		{Chain: Chain{ID: 1, Name: "Ethereum"}, envs: []Environment{Production}},
	}, func() {
		_, err := newChainRegistry(Development)
		if err == nil {
			t.Fatal("expected error when no chain configured for env, got nil")
		}
	})
}

func TestNewChainRegistry_DuplicateChainID(t *testing.T) {
	withChainDefinitions([]chainDef{
		{Chain: Chain{ID: 1, Name: "Ethereum"}, envs: []Environment{Production}},
		{Chain: Chain{ID: 1, Name: "Ethereum Dupe"}, envs: []Environment{Staging}},
	}, func() {
		_, err := newChainRegistry(Production)
		if err == nil {
			t.Fatal("expected error for duplicate chain ID, got nil")
		}
	})
}

func TestNewChainRegistry_MultipleChainsSameEnv(t *testing.T) {
	withChainDefinitions([]chainDef{
		{Chain: Chain{ID: 1, Name: "Ethereum"}, envs: []Environment{Production}},
		{Chain: Chain{ID: 137, Name: "Polygon"}, envs: []Environment{Production}},
	}, func() {
		_, err := newChainRegistry(Production)
		if err == nil {
			t.Fatal("expected error when multiple chains configured for one env, got nil")
		}
	})
}

func TestNewChainRegistry_PopulatesAllChains(t *testing.T) {
	r, err := newChainRegistry(Development)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(r.byID) != len(chainDefs) {
		t.Errorf("expected byID to contain %d chains, got %d", len(chainDefs), len(r.byID))
	}
}

func TestCurrent_ReturnsSingleActiveChain(t *testing.T) {
	r, err := newChainRegistry(Production)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	c := r.Current()
	if c.ID != 137 {
		t.Errorf("expected chain ID 137, got %d", c.ID)
	}
}

func TestByID_KnownChain(t *testing.T) {
	r, _ := newChainRegistry(Development)

	c, ok := r.ByID(137)
	if !ok {
		t.Fatal("expected ByID(137) to return true for known chain")
	}

	if c.Name != "Polygon Mainnet" {
		t.Errorf("expected Polygon Mainnet, got %q", c.Name)
	}
}

func TestByID_UnknownChain(t *testing.T) {
	r, _ := newChainRegistry(Development)

	_, ok := r.ByID(9999)
	if ok {
		t.Error("expected ByID(9999) to return false for unknown chain")
	}
}

func TestValidate_CurrentChain(t *testing.T) {
	r, _ := newChainRegistry(Production)

	c, err := r.Validate(137)
	if err != nil {
		t.Fatalf("unexpected error validating current chain: %v", err)
	}

	if c.ID != 137 {
		t.Errorf("expected chain ID 137, got %d", c.ID)
	}
}

func TestValidate_WrongEnv_KnownChain(t *testing.T) {
	r, _ := newChainRegistry(Production)

	_, err := r.Validate(11155111)
	if err == nil {
		t.Fatal("expected error when validating chain from wrong env, got nil")
	}
}

func TestValidate_UnknownChain(t *testing.T) {
	r, _ := newChainRegistry(Production)

	_, err := r.Validate(9999)
	if err == nil {
		t.Fatal("expected error for unknown chain ID, got nil")
	}
}

func TestAll_ReturnsEveryDefinedChain(t *testing.T) {
	r, _ := newChainRegistry(Development)

	all := r.All()
	if len(all) != len(chainDefs) {
		t.Errorf("expected %d chains from All(), got %d", len(chainDefs), len(all))
	}
}
