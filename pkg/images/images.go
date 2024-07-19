package images

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

func walkMap(m any, cb func(map[any]any)) {
	switch value := m.(type) {
	case map[any]any:
		cb(value)

		for _, v := range value {
			walkMap(v, cb)
		}
	case []any:
		for _, v := range value {
			walkMap(v, cb)
		}
	}
}

func GetImagesFromValuesContent(data []byte) (map[string][]string, error) {
	var values map[any]any
	if err := yaml.Unmarshal(data, &values); err != nil {
		return nil, fmt.Errorf("failed to unmarshal values file for chart: %w", err)
	}

	images := make(map[string][]string)

	walkMap(values, func(m map[any]any) {
		repo, ok := m["repository"].(string)
		if !ok {
			return
		}

		tag, ok := m["tag"].(string)
		if !ok {
			return
		}

		images[repo] = append(images[repo], tag)
	})

	return images, nil
}

func GetImagesFromValuesFile(path string) (map[string][]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read values file for chart: %w", err)
	}

	return GetImagesFromValuesContent(data)
}

func GetImagesFromChart(path string) (map[string][]string, error) {
	valuesFile := filepath.Join(path, "values.yaml")
	return GetImagesFromValuesFile(valuesFile)
}
