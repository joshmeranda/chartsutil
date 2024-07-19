package images_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/joshmeranda/chartsutil/pkg/images"
)

func assertMirrorRefSlice(t *testing.T, expected []images.MirrorRef, actual []images.MirrorRef) {
	t.Helper()

	if len(expected) != len(actual) {
		t.Errorf("expected %d refs, got %d", len(expected), len(actual))
	}

	for _, expectedImage := range expected {
		if !slices.Contains(actual, expectedImage) {
			t.Errorf("expected image %v not found in actual list:\nexpected: %v\n  actual: %v", expectedImage, expected, actual)
			return
		}
	}
}

func TestUnarshalImagesList(t *testing.T) {
	data := []byte(`# this is a comment that should be ignored
rancher/rancher rancher/mirrored-rancher-rancher v2.9.0
library/busybox rancher/mirrored-library-busybox 1.36.1

quay.io/coreos/prometheus-operator rancher/mirrored-coreos-prometheus-operator v0.40.0
registry.k8s.io/metrics-server/metrics-server rancher/mirrored-metrics-server v0.7.1
`)

	list, err := images.UnmarshalImagesList(data)
	if err != nil {
		t.Errorf("recevied unexpected error marshing data: %s", err.Error())
	}

	expected := []images.MirrorRef{
		{Source: "rancher/rancher", Destination: "rancher/mirrored-rancher-rancher", Tag: "v2.9.0"},
		{Source: "library/busybox", Destination: "rancher/mirrored-library-busybox", Tag: "1.36.1"},
		{Source: "quay.io/coreos/prometheus-operator", Destination: "rancher/mirrored-coreos-prometheus-operator", Tag: "v0.40.0"},
		{Source: "registry.k8s.io/metrics-server/metrics-server", Destination: "rancher/mirrored-metrics-server", Tag: "v0.7.1"},
	}

	assertMirrorRefSlice(t, expected, list)
}

func TestMirrorForImage(t *testing.T) {
	type TestCase struct {
		Name string

		Namespace  string
		Repository string
		Tag        string

		Expected images.MirrorRef
		Error    error
	}

	testCases := []TestCase{
		{
			Name: "BasicCase",

			Namespace:  "rancher",
			Repository: "rancher/rancher",
			Tag:        "v2.9.0",

			Expected: images.MirrorRef{
				Source:      "rancher/rancher",
				Destination: "rancher/mirrored-rancher-rancher",
				Tag:         "v2.9.0",
			},
		},
		{
			Name: "WithSourceRegistry",

			Namespace:  "rancher",
			Repository: "some.registry/rancher/rancher",
			Tag:        "v2.9.0",

			Expected: images.MirrorRef{
				Source:      "some.registry/rancher/rancher",
				Destination: "rancher/mirrored-rancher-rancher",
				Tag:         "v2.9.0",
			},
		},
		{
			Name: "NoRepositoryNamespace",

			Namespace:  "rancher",
			Repository: "rancher",
			Tag:        "v2.9.0",

			Error: fmt.Errorf("repository rancher does not contain a namespace"),
		},
		{
			Name: "TooManyComponents",

			Namespace:  "rancher",
			Repository: "rancher/rancher/rancher/rancher",
			Tag:        "v2.9.0",

			Error: fmt.Errorf("repository 'rancher/rancher/rancher/rancher' has too many components"),
		},
	}

	for _, c := range testCases {
		t.Run(c.Name, func(t *testing.T) {
			actual, err := images.MirrorForImage(c.Namespace, c.Repository, c.Tag)
			if err != nil {
				if err.Error() != c.Error.Error() {
					t.Errorf("did not receive the expected error:\nexpected: %v\n  actual: %v", c.Error, err)
				}
			}

			if actual != c.Expected {
				t.Errorf("unexpected mirror for repository:\nexpected: %v\n  actual: %v", c.Expected, actual)
			}
		})
	}
}

func TestGetMissingMirrors(t *testing.T) {
	imageList := images.ImageList{
		"rancher/rancher":          {"v2.9.0"},
		"upstream/subcomponent":    {"v0.0.3"},
		"upstream/subsubcomponent": {"v0.0.3"},
		"upstream/something-new":   {"v0.0.0"},
	}

	mirrors := []images.MirrorRef{
		{Source: "rancher/rancher", Destination: "rancher/mirrored-rancher-rancher", Tag: "v2.8.0"},
		{Source: "upstream/subcomponent", Destination: "rancher/mirrored-upstream-subcomponent", Tag: "v0.0.0"},
		{Source: "upstream/subcomponent", Destination: "rancher/mirrored-upstream-subcomponent", Tag: "v0.0.1"},
		{Source: "upstream/subsubcomponent", Destination: "rancher/mirrored-upstream-subsubcomponent", Tag: "v0.0.3"},
	}

	expected := []images.MirrorRef{
		{Source: "rancher/rancher", Destination: "rancher/mirrored-rancher-rancher", Tag: "v2.9.0"},
		{Source: "upstream/subcomponent", Destination: "rancher/mirrored-upstream-subcomponent", Tag: "v0.0.3"},
		{Source: "upstream/something-new", Destination: "rancher/mirrored-upstream-something-new", Tag: "v0.0.0"},
	}

	actual, err := images.GetMissingMirrorRefs("rancher", imageList, mirrors)
	if err != nil {
		t.Errorf("unexpected error: %s", err.Error())
	}

	assertMirrorRefSlice(t, expected, actual)
}
