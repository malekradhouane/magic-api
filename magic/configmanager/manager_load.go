package configmanager

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	kfile "github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

const (
	PathConfig = "config"
	PathMagic  = "magic"
)

const (
	NoConfigFound = "no configuration file found"
)

// loadAll loads all configurations.
func (cman *ManagerWithKoanf) loadAll() error {
	var (
		log          = cman.logger
		confProvider = cman.configProvider
	)

	cman.mu.Lock()
	defer cman.mu.Unlock()

	if err := cman.loadConfigFromGlob(); err != nil {
		log.Error(err.Error())
		return err
	}

	if err := cman.loadFromEnvVar(cman.configProvider); err != nil {
		log.Error(err.Error())
		return err
	}

	// Map configProvider internals data to an Magic config struct.
	if err := confProvider.Unmarshal("", &cman.magicConfig); err != nil {
		log.Error(err.Error(), "konf.Raw()", confProvider.Raw())
		return err
	}

	return nil
}

func (cman *ManagerWithKoanf) loadConfigFromGlob() error {
	var (
		//environment = cman.Environment()
		log = cman.logger
	)

	//if environment != "" {
	//	searchPath := filepath.Join(cman.configRootDir, PathEnvConfig, environment, PathMagic, "*.yaml")
	//	nb, err := cman.loadFromGlob(cman.configProvider, searchPath)
	//	if err != nil {
	//		return err
	//	}
	//	if nb == 0 {
	//		log.Info(NoConfigFound)
	//	}
	//} else {
	//	log.Info("customer warehouse [environment] is not set. Skipping.")
	//}

	// Read config from $(pwd)/config/magic.yaml only.
	// This avoids unintentionally loading other YAML files (e.g., storeinit configs)
	// that may define conflicting schemas like 'storages' with a different type.
	searchPath := filepath.Join(cman.configRootDir, PathConfig, PathMagic+".yaml")
	nb, err := cman.loadFromGlob(cman.configProvider, searchPath)
	if err != nil {
		log.Error(err.Error())
		return err
	}
	if nb == 0 {
		log.Info(NoConfigFound)
	}

	return nil
}

func (cman *ManagerWithKoanf) loadFromEnvVar(konf *koanf.Koanf) error {
	skipMe := []string{}

	return konf.Load(
		env.Provider("", ".",
			func(s string) string {
				if slices.Contains(skipMe, s) {
					return "" // ignore
				}
				s = strings.TrimPrefix(s, "MAGIC_")
				s = strings.ReplaceAll(s, "_", ".")
				s = strings.ToLower(s)

				return s
			}),
		nil)
}

// loadFromGlob returns number of loaded files
func (cman *ManagerWithKoanf) loadFromGlob(konf *koanf.Koanf, globPaths ...string) (int, error) {
	var (
		log      = cman.logger
		nbLoaded = 0
	)

	if len(globPaths) == 0 {
		return nbLoaded, nil
	}

	for _, globPath := range globPaths {
		path, err := filepath.Abs(globPath)
		if err != nil {
			log.Error(err.Error())
			return nbLoaded, err
		}
		log.Info("searching configuration ...", "path", path)

		fileParser := yaml.Parser()
		filesToLoad, err := filepath.Glob(path)
		if err != nil {
			log.Error(err.Error())
			return nbLoaded, err
		}

		for _, file := range filesToLoad {
			provider := kfile.Provider(file)
			if err := konf.Load(provider, fileParser); err != nil {
				log.Error(err.Error())
				return nbLoaded, fmt.Errorf("%w: %w", ErrLoadingConfiguration, err)
			}
			nbLoaded++

			log.Info("configuration loaded", "file", file)
		}
	}

	return nbLoaded, nil
}
