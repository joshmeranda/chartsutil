package images_test

import (
	"slices"
	"testing"

	"github.com/joshmeranda/chartsutil/pkg/images"
)

var content []byte = []byte(`
name: some-app
port: 8080

image:
  repository: rancher/rancher
  tag: v2.9.0

tolerations: []
taints: []

oldImages:
  - repository: rancher/rancher
    tag: v2.8.0
  - repository: rancher/rancher
    tag: v2.7.0
  - repository: rancher/rancher
    tag: v2.6.0
  - repository: rancher/rancher
    tag: v2.5.0

subComponent:
  image:
    repository: upstream/subcomponent
    tag: ""

subSubComponent:
  image:
    repository: upstream/subsubcomponent
    tag: ""
`)

func assertImageMaps(t *testing.T, actual, expected map[string][]string) {
	t.Helper()
	if len(actual) != len(expected) {
		t.Errorf("actual does not match expected:\nExpected: %v\n  Actual: %v", expected, actual)
		return
	}

	for ek, ev := range expected {
		av, ok := actual[ek]
		if !ok {
			t.Errorf("actual does not match expected:\nExpected: %v\n  Actual: %v", expected, actual)
		}

		slices.Sort(ev)
		slices.Sort(av)

		if !slices.Equal(ev, av) {
			t.Errorf("actual does not match expected:\nExpected: %v\n  Actual: %v", expected, actual)
			return
		}
	}
}

func TestGetImagesFromValuesContent(t *testing.T) {
	imageMap, err := images.GetImagesFromValuesContent(content)
	if err != nil {
		t.Error("unexpected error:", err)
	}

	expected := map[string][]string{
		"rancher/rancher":          {"v2.9.0", "v2.8.0", "v2.7.0", "v2.6.0", "v2.5.0"},
		"upstream/subcomponent":    {""},
		"upstream/subsubcomponent": {""},
	}

	assertImageMaps(t, imageMap, expected)
}
