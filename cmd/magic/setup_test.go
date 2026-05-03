package main

import (
	"testing"
)

func TestResourcesRegistry_mergeWithEnviron(t *testing.T) {
	// Setup a semi controlled env

	warehouse := "calisson_aix"
	t.Setenv("ENVIRONMENT", warehouse)

	// Test that we found at least both ENVIRONMENT and STOREINIT_CONFIG

	rr := new(ResourcesRegistry)
	userEnv := []string{"STOREINIT_CONFIG=/"}
	newEnv := rr.mergeWithEnviron(userEnv)
	found := 0
	for _, env := range newEnv {
		switch env {
		case "STOREINIT_CONFIG=/":
			found++
		case "ENVIRONMENT=" + warehouse:
			found++
		}
	}
	if found != 2 {
		t.Fatal("cannot found all expected variables")
	}

	// Test with variable expansion

	userEnv = []string{"STOREINIT_CONFIG=/"}
	newEnv = rr.mergeWithEnviron(userEnv)
	found = 0
	for _, env := range newEnv {
		switch env {
		case "STOREINIT_CONFIG=/":
			found++
		case "ENVIRONMENT=":
			found++
		}
	}
	if found != 2 {
		t.Fatal("cannot found all expected variables")
	}

	// Test with fuzzy content in userEnv

	envWithoutFuzzy := rr.mergeWithEnviron([]string{})
	newWithFuzzy := rr.mergeWithEnviron([]string{"FUZZY_DATA", "", "		", "TEST=ok"})
	if len(envWithoutFuzzy) != len(newWithFuzzy)-1 {
		t.Fatal("no fuzzy variable should be present")
	}
}
