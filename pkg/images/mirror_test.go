package images_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/joshmeranda/chartsutil/pkg/images"
)

func assertMirrorRef(t *testing.T, expected images.SourceRef, actual images.SourceRef) bool {
	t.Helper()

	if expected.Source != actual.Source {
		t.Errorf("expected mirror %s, got %s", expected.Source, actual.Source)
		return false
	}

	for _, expectedTag := range expected.Tags {
		if !slices.Contains(actual.Tags, expectedTag) {
			t.Errorf("expected mirror %s, got %s", expected.Source, actual.Source)
			return false
		}
	}

	return true
}

func assertMirrorList(t *testing.T, expected images.MirrorList, actual images.MirrorList) {
	t.Helper()

	if len(expected) != len(actual) {
		t.Errorf("expected %d refs, got %d:\nexpected: %v\n  actual: %v", len(expected), len(actual), expected, actual)
		return
	}

	for expectedImage, expectedRef := range expected {
		actualImage, found := actual[expectedImage]
		if !found {
			t.Errorf("actual mirror list does not match expected:\nexpected: %v\n  actual: %v", expected, actual)
			return
		}

		if !assertMirrorRef(t, expectedRef, actualImage) {
			t.Errorf("actual mirror list does not match expected:\nexpected: %v\n  actual: %v", expected, actual)
			return
		}
	}
}

func TestMirrorListAddMirror(t *testing.T) {
	list := images.MirrorList{}

	list.AddMirror("upstream/subcomponent", "rancher/mirrored-upstream-subcomponent", "v0.0.0")
	list.AddMirror("upstream/subcomponent", "rancher/mirrored-upstream-subcomponent", "v0.0.1")

	expected := images.MirrorList{
		"rancher/mirrored-upstream-subcomponent": images.SourceRef{
			Source: "upstream/subcomponent",
			Tags:   []string{"v0.0.0", "v0.0.1"},
		},
	}

	assertMirrorList(t, expected, list)
}

func TestMirrorListHasMirror(t *testing.T) {
	list := images.MirrorList{
		"rancher/mirrored-upstream-subcomponent": images.SourceRef{
			Source: "upstream/subcomponent",
			Tags:   []string{"v0.0.0", "v0.0.1"},
		},
	}

	if !list.HasMirror("upstream/subcomponent", "rancher/mirrored-upstream-subcomponent", "v0.0.0") {
		t.Errorf("expected mirror to be found")
	}

	if list.HasMirror("upstream/subcomponent", "rancher/mirrored-upstream-subcomponent", "v0.0.2") {
		t.Errorf("expected mirror to not be found")
	}
}

func TestMirrorListDestinationForSource(t *testing.T) {
	list := images.MirrorList{
		"rancher/mirrored-upstream-subcomponent": images.SourceRef{
			Source: "upstream-community/subcomponent",
			Tags:   []string{"v0.0.0", "v0.0.1"},
		},
	}

	actual, found := list.DestinationForSource("upstream-community/subcomponent")
	if !found {
		t.Errorf("expected mirror to be found")
	}
	if actual != "rancher/mirrored-upstream-subcomponent" {
		t.Errorf("expected mirror rancher/mirrored-upstream-subcomponent, got %s", actual)
	}

	_, found = list.DestinationForSource("upstream/subcomponent")
	if found {
		t.Errorf("expected mirror to not be found")
	}
}

func TestUnarshalImagesList(t *testing.T) {
	data := []byte(`# this is a comment that should be ignored
rancher/rancher rancher/mirrored-rancher-rancher v2.9.0
library/busybox rancher/mirrored-library-busybox 1.36.1

quay.io/coreos/prometheus-operator rancher/mirrored-coreos-prometheus-operator v0.40.0
registry.k8s.io/metrics-server/metrics-server rancher/mirrored-metrics-server v0.7.1

jimmidyson/config-reloader rancher/jimmidyson-config-reloader v0.0.1
jimmidyson/config-reloader rancher/mirrored-jimmidyson-config-reloader v0.0.2
`)

	list, err := images.UnmarshalImagesList(data)
	if err != nil {
		t.Errorf("recevied unexpected error marshing data: %s", err.Error())
	}

	expected := images.MirrorList{
		"rancher/mirrored-rancher-rancher":            images.SourceRef{Source: "rancher/rancher", Tags: []string{"v2.9.0"}},
		"rancher/mirrored-library-busybox":            images.SourceRef{Source: "library/busybox", Tags: []string{"1.36.1"}},
		"rancher/mirrored-coreos-prometheus-operator": images.SourceRef{Source: "quay.io/coreos/prometheus-operator", Tags: []string{"v0.40.0"}},
		"rancher/mirrored-metrics-server":             images.SourceRef{Source: "registry.k8s.io/metrics-server/metrics-server", Tags: []string{"v0.7.1"}},
		"rancher/jimmidyson-config-reloader":          images.SourceRef{Source: "jimmidyson/config-reloader", Tags: []string{"v0.0.1"}},
		"rancher/mirrored-jimmidyson-config-reloader": images.SourceRef{Source: "jimmidyson/config-reloader", Tags: []string{"v0.0.2"}},
	}

	assertMirrorList(t, expected, list)
}

func TestMirrorForSource(t *testing.T) {
	type TestCase struct {
		Name   string
		Source string
		Mirror string
		Error  error
	}

	var namespace = "rancher"

	testCases := []TestCase{
		{
			Name:   "NoRegistry",
			Source: "upstream/subcomponent",
			Mirror: "rancher/mirrored-upstream-subcomponent",
		},
		{
			Name:   "WithRegistry",
			Source: "quay.io/upstream/subcomponent",
			Mirror: "rancher/mirrored-upstream-subcomponent",
		},
		{
			Name:   "NoNamespace",
			Source: "subcomponent",
			Error:  fmt.Errorf("repository 'subcomponent' does not contain a namespace"),
		},
		{
			Name:   "TooManyComponents",
			Source: "quay.io/upstream/subcomponent/extra",
			Error:  fmt.Errorf("repository 'quay.io/upstream/subcomponent/extra' has too many components"),
		},
	}

	for _, c := range testCases {
		t.Run(c.Name, func(t *testing.T) {
			actual, err := images.MirrorForSource(namespace, c.Source)
			if err != nil && err.Error() != c.Error.Error() {

				t.Errorf("expected error '%s', got '%s'", c.Error, err)
				return
			}

			if actual != c.Mirror {
				t.Errorf("expected mirror '%s', got '%s'", c.Mirror, actual)
				return
			}
		})
	}
}

func TestGetMissingMirrors(t *testing.T) {
	imageList := images.ImageList{
		"rancher/fluent-bit":                     {"2.2.0"},
		"upstream/subcomponent":                  {"v0.0.3"},
		"upstream/subsubcomponent":               {"v0.0.3"},
		"upstream/something-new":                 {"v0.0.0"},
		"rancher/mirrored-upstream-subcomponent": {"v0.0.4"},
	}

	mirrors := images.MirrorList{
		"rancher/mirrored-upstream-subcomponent":    images.SourceRef{Source: "upstream/subcomponent", Tags: []string{"v0.0.0", "v0.0.1"}},
		"rancher/mirrored-upstream-subsubcomponent": images.SourceRef{Source: "upstream/subsubcomponent", Tags: []string{"v0.0.3"}},
	}

	expected := images.MirrorList{
		"rancher/mirrored-upstream-subcomponent":  images.SourceRef{Source: "upstream/subcomponent", Tags: []string{"v0.0.3", "v0.0.4"}},
		"rancher/mirrored-upstream-something-new": images.SourceRef{Source: "upstream/something-new", Tags: []string{"v0.0.0"}},
	}

	actual, err := images.GetMissingMirrorRefs("rancher", imageList, mirrors)
	if err != nil {
		t.Errorf("unexpected error: %s", err.Error())
	}

	assertMirrorList(t, expected, actual)
}

func TestGetMissingMirrorsWithDifferingDestination(t *testing.T) {
	imageList := images.ImageList{
		"upstream-community/subcomponent": {"v0.0.2"},
	}

	mirrors := images.MirrorList{
		"rancher/mirrored-upstream-subcomponent": images.SourceRef{Source: "upstream-community/subcomponent", Tags: []string{"v0.0.0", "v0.0.1"}},
	}

	expected := images.MirrorList{
		"rancher/mirrored-upstream-subcomponent": images.SourceRef{Source: "upstream-community/subcomponent", Tags: []string{"v0.0.2"}},
	}

	actual, err := images.GetMissingMirrorRefs("rancher", imageList, mirrors)
	if err != nil {
		t.Errorf("unexpected error: %s", err.Error())
	}

	assertMirrorList(t, expected, actual)
}

func TestRepositoryIsMirror(t *testing.T) {
	type TestCase struct {
		Name       string
		Repository string
		Expected   bool
	}

	testCases := []TestCase{
		{
			Name:       "IsMirror",
			Repository: "rancher/mirrored-rancher-rancher",
			Expected:   true,
		},
		{
			Name:       "IsNotMirror",
			Repository: "rancher/rancher",
		},
		{
			Name:       "HasRegistry",
			Repository: "quay.io/upstream/mirrored-subcomponent",
		},
	}

	for _, c := range testCases {
		t.Run(c.Name, func(t *testing.T) {
			actual := images.RepositoryIsMirror(c.Repository)
			if actual != c.Expected {
				t.Errorf("expected '%v' but found '%v' for '%s'", c.Expected, actual, c.Repository)
			}
		})
	}
}
