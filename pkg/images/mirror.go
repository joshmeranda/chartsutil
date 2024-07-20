package images

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
)

// MirrorRef is a struct that represents each mirror from the rancher/image-mirror images-list file.
// todo: might be a good idea to make this an ImageList
type MirrorRef struct {
	// Source is the source image excluding the registry.
	Source      string
	Destination string
	Tag         string
}

func (r MirrorRef) String() string {
	return fmt.Sprintf("%s %s %s", r.Source, r.Destination, r.Tag)
}

func UnmarshalImagesList(data []byte) ([]MirrorRef, error) {
	reader := bytes.NewReader(data)
	scanner := bufio.NewScanner(reader)

	out := make([]MirrorRef, 0)

	for i := 1; scanner.Scan(); i++ {
		line := scanner.Bytes()

		if len(line) == 0 || bytes.HasPrefix(line, []byte("#")) {
			continue
		}

		components := bytes.Split(line, []byte(" "))
		if len(components) != 3 {
			return nil, fmt.Errorf("error line line %d: expected 3 fields but only found %d", i, len(components))
		}

		out = append(out, MirrorRef{
			Source:      string(components[0]),
			Destination: string(components[1]),
			Tag:         string(components[2]),
		})
	}

	return out, nil
}

func MirrorForSource(namespace string, repository string, tag string) (MirrorRef, error) {
	components := strings.Split(repository, "/")

	var oldNamespace, name string

	switch len(components) {
	case 1:
		return MirrorRef{}, fmt.Errorf("repository %s does not contain a namespace", repository)
	case 2:
		oldNamespace, name = components[0], components[1]
	case 3:
		oldNamespace, name = components[1], components[2]
	default:
		return MirrorRef{}, fmt.Errorf("repository '%s' has too many components", repository)
	}

	return MirrorRef{
		Source:      repository,
		Destination: fmt.Sprintf("%s/mirrored-%s-%s", namespace, oldNamespace, name),
		Tag:         tag,
	}, nil
}

// todo: should support different namespaces
func GetMissingMirrorRefs(namespace string, images ImageList, mirrors []MirrorRef) ([]MirrorRef, error) {
	var newMirrors []MirrorRef

	for repository, tags := range images {
		for _, tag := range tags {
			missing := true
			var existingMirror *MirrorRef

			for _, ref := range mirrors {
				if (ref.Source == repository || ref.Destination == repository) && ref.Tag == tag {
					missing = false
					break
				}

				if ref.Destination == repository {
					existingMirror = &ref
					break
				}
			}

			if missing {
				var newMirror MirrorRef
				var err error

				if existingMirror == nil {
					newMirror, err = MirrorForSource(namespace, repository, tag)
					if err != nil {
						return nil, fmt.Errorf("failed to create mirror for source: %w", err)
					}
				} else {
					newMirror = *existingMirror
					newMirror.Tag = tag
				}

				newMirrors = append(newMirrors, newMirror)
			}
		}
	}

	return newMirrors, nil
}
