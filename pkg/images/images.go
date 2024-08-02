package images

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
	"k8s.io/utils/strings/slices"
)

type ImageList map[string][]string

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

func GetImagesFromValuesContent(data []byte) (ImageList, error) {
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

		// todo: can probably do this more efficiently, but this will be fine for most cases
		if !slices.Contains(images[repo], tag) {
			images[repo] = append(images[repo], tag)
		}
	})

	return images, nil
}

func GetImagesFromValuesFile(path string) (ImageList, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read values file for chart: %w", err)
	}

	return GetImagesFromValuesContent(data)
}

func GetImagesFromChart(path string) (ImageList, error) {
	valuesFile := filepath.Join(path, "values.yaml")
	imagesList, err := GetImagesFromValuesFile(valuesFile)
	if err != nil {
		return nil, fmt.Errorf("failed to get images from main chart: %w", err)
	}

	subchartDir := filepath.Join(path, "charts")
	entries, err := os.ReadDir(subchartDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("failed to read subchart directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			subchartPath := filepath.Join(subchartDir, entry.Name())
			subchartImages, err := GetImagesFromChart(subchartPath)
			if err != nil {
				return nil, fmt.Errorf("failed to get images from subchart '%s': %w", entry.Name(), err)
			}

			for repo, tags := range subchartImages {
				imagesList[repo] = slices.Filter(imagesList[repo], tags, func(tag string) bool {
					return !slices.Contains(imagesList[repo], tag)
				})
			}
		}
	}

	return imagesList, nil
}

func RepositoryInNamespace(repository string, namespace string) bool {
	components := strings.Split(repository, "/")

	switch len(components) {
	case 2:
		return components[0] == namespace
	case 3:
		return components[1] == namespace
	default:
		return false
	}
}
