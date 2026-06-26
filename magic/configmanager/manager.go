package configmanager

// Contracts

type ManagerContract interface {
	// Get Magic config struct
	Magic() *Magic

	// Get current environment (AKA customer warehouse)
	Environment() string

	// Get customer specific parameter.
	CustomerParam(key string) (any, bool)
}

// Dependencies

type ManagerLogger interface {
	Error(msg string, args ...any)
	Info(msg string, args ...any)
	Debug(msg string, args ...any)
	Warn(msg string, args ...any)
}

// Params

type ManagerParams struct {
	// AKA customer warehouse.
	// Could be overridden by env. var. (cf README.md)
	// Default is "dummy".
	Environment string

	// Path from which to read configuration files.
	// Could be overridden by env. var. (cf README.md)
	// Default is current working directory.
	ConfigRootDir string // Path to read configuration files from.
}
