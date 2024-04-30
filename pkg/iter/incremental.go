package iter

import (
	"errors"
	"fmt"
	"io"

	"github.com/rancher/charts-build-scripts/pkg/charts"
	"github.com/rancher/charts-build-scripts/pkg/puller"
)

type ForEachFunc func(p puller.Puller) error

func ForEach(iter UpstreamIter, fn ForEachFunc) error {
	for {
		p, err := iter.Next()
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return fmt.Errorf("encoutnered error getting next puller: %w", err)
		}

		if err := fn(p); err != nil {
			return fmt.Errorf("ForEachFunc returned with err: %w", err)
		}
	}

	return nil
}

type UpstreamIter interface {
	// Next returns the next puller in the iterator and points the head at the next item. If empty returns io.EOF.
	Next() (puller.Puller, error)
}

type SingleIter struct {
	Upstream puller.Puller
}

func (i *SingleIter) Next() (puller.Puller, error) {
	if i.Upstream == nil {
		return i.Upstream, io.EOF
	}

	p := i.Upstream
	i.Upstream = nil

	return p, nil
}

func IterForUpstream(upstream puller.Puller, target string) (UpstreamIter, error) {
	if target == "" {
		return nil, fmt.Errorf("target cannot be empty")
	}

	switch u := upstream.(type) {
	case puller.GithubRepository:
		return NewGitIter(u.GetOptions(), target)
	default:
		return NewSingleIter(upstream, target)
	}
}

func NewSingleIter(upstream puller.Puller, target string) (UpstreamIter, error) {
	opts := upstream.GetOptions()

	switch upstream.(type) {
	case puller.GithubRepository:
		opts.Commit = &target
	default:
		opts.URL = target
	}

	p, err := charts.GetUpstream(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get upstream: %w", err)
	}

	return &SingleIter{
		Upstream: p,
	}, nil
}