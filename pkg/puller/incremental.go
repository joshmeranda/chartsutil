package puller

import "github.com/rancher/charts-build-scripts/pkg/puller"

type IncrementalPuller struct {
	Pullers []puller.Puller

	i int
}

func (ip *IncrementalPuller) Next() (puller.Puller, bool) {
	if len(ip.Pullers) == 0 || ip.i >= len(ip.Pullers) {
		return nil, false
	}

	next := ip.Pullers[ip.i]
	ip.i++

	return next, true
}

type PullerIter interface {
	Next() (puller.Puller, bool)
	ForEach(func(puller.Puller) error) error
}
