package images

import (
	"bufio"
	"bytes"
	"fmt"
)

// MirrorRef is a struct that represents each mirror from the rancher/image-mirror images-list file.
type MirrorRef struct {
	// Source is the source image excluding the registry.
	Source      string
	Destination string
	Tag         string
}

func (r MirrorRef) String() string {
	return fmt.Sprintf("%s %s %s", r.Source, r.Destination, r.Tag)
}

func MarshalImagesList(data []byte) ([]MirrorRef, error) {
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
