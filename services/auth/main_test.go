package main

import "testing"

func TestDoSomething(t *testing.T) {
	result := doSomething(1, 2)
	if result != 3 {
		t.Error("Expected 3, got ", result)
	}
}
