package images_test

import (
	"slices"
	"testing"

	"github.com/joshmeranda/chartsutil/pkg/images"
)

func assertImagesList(t *testing.T, expected, actual []images.MirrorRef) {
	t.Helper()

	if len(expected) != len(actual) {
		t.Errorf("expected %d images, got %d", len(expected), len(actual))
	}

	for _, expectedImage := range expected {
		if !slices.Contains(actual, expectedImage) {
			t.Errorf("expected image %v not found in actual list:\nexpected: %v\n  actual: %v", expectedImage, expected, actual)
			return
		}
	}
}

func TestMarshalImagesList(t *testing.T) {
	data := []byte(`# this is a comment that should be ignored
rancher/rancher rancher/mirrored-rancher-rancher v2.9.0
library/busybox rancher/mirrored-library-busybox 1.36.1

quay.io/coreos/prometheus-operator rancher/mirrored-coreos-prometheus-operator v0.40.0
registry.k8s.io/metrics-server/metrics-server rancher/mirrored-metrics-server v0.7.1
`)

	list, err := images.MarshalImagesList(data)
	if err != nil {
		t.Errorf("recevied unexpected error marshing data: %s", err.Error())
	}

	expected := []images.MirrorRef{
		{Source: "rancher/rancher", Destination: "rancher/mirrored-rancher-rancher", Tag: "v2.9.0"},
		{Source: "library/busybox", Destination: "rancher/mirrored-library-busybox", Tag: "1.36.1"},
		{Source: "quay.io/coreos/prometheus-operator", Destination: "rancher/mirrored-coreos-prometheus-operator", Tag: "v0.40.0"},
		{Source: "registry.k8s.io/metrics-server/metrics-server", Destination: "rancher/mirrored-metrics-server", Tag: "v0.7.1"},
	}

	assertImagesList(t, expected, list)
}
