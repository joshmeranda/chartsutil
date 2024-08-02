package images

import (
	"bufio"
	"bytes"
	"fmt"
	"slices"
	"strings"
)

// MirrorRef is a struct that represents each mirror from the rancher/image-mirror images-list file.
// todo: might be a good idea to make this an ImageList
type SourceRef struct {
	Source string
	Tags   []string
}

// MirrorList is a map of mirror names to source names and tags.
type MirrorList map[string]SourceRef

func (m MirrorList) AddMirror(source string, destination string, tag string) {
	sourceRef, found := m[destination]

	if !found {
		m[destination] = SourceRef{
			Source: source,
			Tags:   []string{tag},
		}
	} else {
		sourceRef.Tags = append(sourceRef.Tags, tag)
		m[destination] = sourceRef
	}
}

func (m MirrorList) HasMirror(source string, destination string, tag string) bool {
	sourceRef, found := m[destination]

	if !found {
		return false
	}

	if sourceRef.Source == source && slices.Contains(sourceRef.Tags, tag) {
		return true
	}

	return false
}

func (m MirrorList) DestinationForSource(source string) (string, bool) {
	for destination, sourceRef := range m {
		if sourceRef.Source == source {
			return destination, true
		}
	}

	return "", false
}

func UnmarshalImagesList(data []byte) (MirrorList, error) {
	reader := bytes.NewReader(data)
	scanner := bufio.NewScanner(reader)

	out := make(MirrorList, 0)

	for i := 1; scanner.Scan(); i++ {
		line := scanner.Bytes()

		if len(line) == 0 || bytes.HasPrefix(line, []byte("#")) {
			continue
		}

		components := bytes.Split(line, []byte(" "))
		if len(components) != 3 {
			return nil, fmt.Errorf("error line line %d: expected 3 fields but only found %d", i, len(components))
		}

		source, destination, tag := string(components[0]), string(components[1]), string(components[2])
		out.AddMirror(source, destination, tag)
	}

	return out, nil
}

func MirrorForSource(namespace string, source string) (string, error) {
	components := strings.Split(source, "/")

	var oldNamespace, name string

	switch len(components) {
	case 1:
		return "", fmt.Errorf("repository '%s' does not contain a namespace", source)
	case 2:
		oldNamespace, name = components[0], components[1]
	case 3:
		oldNamespace, name = components[1], components[2]
	default:
		return "", fmt.Errorf("repository '%s' has too many components", source)
	}

	return fmt.Sprintf("%s/mirrored-%s-%s", namespace, oldNamespace, name), nil
}

func RepositoryIsMirror(repository string) bool {
	components := strings.Split(repository, "/")

	if len(components) < 2 {
		return false
	}

	return strings.HasPrefix(components[1], "mirrored-")
}

func GetMissingMirrorRefs(namespace string, images ImageList, mirrors MirrorList) (MirrorList, error) {
	var err error

	newMirrors := MirrorList{}

	for repository, tags := range images {
		if RepositoryIsMirror(repository) {
			source, found := mirrors[repository]
			if !found {
				return nil, fmt.Errorf("mirror '%s' is not in the mirror list", repository)
			}
			repository = source.Source
		} else if RepositoryInNamespace(repository, namespace) {
			continue
		}

		mirror, found := mirrors.DestinationForSource(repository)

		if !found {
			mirror, err = MirrorForSource(namespace, repository)
			if err != nil {
				return nil, fmt.Errorf("failed to create mirror for source repository '%s': %w", repository, err)
			}
		}

		for _, tag := range tags {
			if !mirrors.HasMirror(repository, mirror, tag) {
				newMirrors.AddMirror(repository, mirror, tag)
			}
		}
	}

	return newMirrors, nil
}
