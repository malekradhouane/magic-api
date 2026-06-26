package configmanager

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMain sets up the environment, runs the tests, and performs cleanup.
func TestMain(m *testing.M) {
	m.Run()
}

func TestManagerWithTestConfig(t *testing.T) {
	testConfig := &Magic{
		Customer: map[string]any{"titi": "toto"},
	}

	cman, err := DefaultManagerWithKonf().
		WithMagicConfig(testConfig).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	itConfig := cman.Magic()

	if itConfig.Customer["titi"] != "toto" {
		t.Fail()
	}

	//if itConfig.Mode != "PROD" {
	//	t.Fail()
	//}
}

func TestManagerWithDataset1(t *testing.T) {
	t.Skip("dataset fixtures are intentionally ignored in CI")

	const (
		dataset = "dataset1"
		env     = "dummy"
	)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	cman, err := DefaultManagerWithKonf().
		WithEnvironment(env).
		WithConfigRoot(filepath.Join(cwd, "test", dataset)).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	processAssertions(t, env, cman)
}

func TestManagerWithDataset2(t *testing.T) {
	t.Skip("dataset fixtures are intentionally ignored in CI")

	const (
		dataset = "dataset2"
		env     = "dummy"
	)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	cman, err := DefaultManagerWithKonf().
		WithEnvironment(env).
		WithConfigRoot(filepath.Join(cwd, "test", dataset)).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	processAssertions(t, env, cman)
}

func processAssertions(t *testing.T, env string, cman ManagerContract) {
	conf := cman.Magic()
	cmanEnv := cman.Environment()

	// Ensure that the global configuration is loaded correctly
	assert.NotNil(t, conf, "Global configuration should not be nil")
	assert.Equal(t, env, cmanEnv)

	// Verify HTTP server settings
	assert.Equal(t, uint(5000), conf.HttpServer.Port)
	assert.False(t, conf.HttpServer.TLS)

	// Check logging configuration
	assert.Equal(t, "error", conf.Logging.Level)
	assert.Equal(t, "json", conf.Logging.Formatter)
	assert.False(t, conf.Logging.Verbose)

	// Validate the operational mode
	assert.Equal(t, ModeProd, conf.Mode)

	val, found := cman.CustomerParam("TEST")
	assert.True(t, found)
	assert.Equal(t, val, "VALID")
}
