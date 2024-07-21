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
type MirrorRef struct {
	// Source is the source image excluding the registry.
	Mirror string
	Tags   []string
}

type MirrorList map[string]MirrorRef

func (m MirrorList) SourceForMirror(mirror string) (string, bool) {
	for source, ref := range m {
		if ref.Mirror == mirror {
			return source, true
		}
	}

	return "", false
}

func (m MirrorList) HasMirror(repository string, tag string) bool {
	ref, found := m[repository]

	if !found {
		return false
	}

	return slices.Contains(ref.Tags, tag)
}

func (m MirrorList) AddMirror(repository string, tag string, mirror string) {
	ref, found := m[repository]

	if !found {
		m[repository] = MirrorRef{
			Mirror: mirror,
			Tags:   []string{tag},
		}
	} else {
		ref.Tags = append(ref.Tags, tag)
		m[repository] = ref
	}
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

		mirror, found := out[string(components[0])]

		if !found {
			out[string(components[0])] = MirrorRef{
				Mirror: string(components[1]),
				Tags:   []string{string(components[2])},
			}
		} else {
			mirror.Tags = append(mirror.Tags, string(components[2]))
		}
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

// func MirrorForSource(namespace string, repository string, tag string) (MirrorRef, error) {
// 	components := strings.Split(repository, "/")
//
// 	var oldNamespace, name string
//
// 	switch len(components) {
// 	case 1:
// 		return MirrorRef{}, fmt.Errorf("repository %s does not contain a namespace", repository)
// 	case 2:
// 		oldNamespace, name = components[0], components[1]
// 	case 3:
// 		oldNamespace, name = components[1], components[2]
// 	default:
// 		return MirrorRef{}, fmt.Errorf("repository '%s' has too many components", repository)
// 	}
//
// 	return MirrorRef{
// 		Source:      repository,
// 		Destination: fmt.Sprintf("%s/mirrored-%s-%s", namespace, oldNamespace, name),
// 		Tags:        []string{tag},
// 	}, nil
// }

func RepositoryIsMirror(repository string) bool {
	components := strings.Split(repository, "/")

	if len(components) < 2 {
		return false
	}

	return strings.HasPrefix(components[1], "mirrored-")
}

func GetMissingMirrorRefs(namespace string, images ImageList, mirrors MirrorList) (MirrorList, error) {
	newMirrors := MirrorList{}

	var found bool

	for repository, tags := range images {
		isMirror := RepositoryIsMirror(repository)
		if isMirror {
			repository, found = mirrors.SourceForMirror(repository)
			if !found {
				return nil, fmt.Errorf("mirror '%s' not found in mirrors list", repository)
			}
		}

		for _, tag := range tags {
			if mirrors.HasMirror(repository, tag) {
				continue
			}

			mirrorName, err := MirrorForSource(namespace, repository)
			if err != nil {
				return nil, fmt.Errorf("failed to create mirror for source: %w", err)
			}

			newMirrors.AddMirror(repository, tag, mirrorName)
		}
	}

	return newMirrors, nil
}
