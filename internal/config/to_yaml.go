package config

import (
	"fmt"
	"os"

	"github.com/go-viper/mapstructure/v2"
	"gopkg.in/yaml.v3"
)

func ToYAMLFile(config any, filename string) error {
	var mapData map[string]any

	err := mapstructure.Decode(config, &mapData)
	if err != nil {
		return fmt.Errorf("failed to decode config to map: %w", err)
	}

	yamlData, err := yaml.Marshal(mapData)
	if err != nil {
		return fmt.Errorf("failed to marshal map to yaml: %w", err)
	}

	err = os.WriteFile(filename, yamlData, 0600)
	if err != nil {
		return fmt.Errorf("failed to write yaml to file: %w", err)
	}

	return nil
}
