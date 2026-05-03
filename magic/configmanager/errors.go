package configmanager

import (
	"errors"
	"fmt"
)

var (
	ErrConfigManager        = errors.New("config manager")
	ErrLoadingConfiguration = fmt.Errorf("%w: loading configuration", ErrConfigManager)
)
