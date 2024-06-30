package iter

import (
	"errors"
	"fmt"
	"io"

	"github.com/rancher/charts-build-scripts/pkg/charts"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/puller"
)

// UpstreamDelta showing the changes to make when creating the next upstreams in an iterator.
type UpstreamDelta options.UpstreamOptions

func (d *UpstreamDelta) Apply(opts options.UpstreamOptions) options.UpstreamOptions {
	if d.Commit != nil {
		opts.Commit = d.Commit
	}

	if d.URL != "" {
		opts.URL = d.URL
	}

	if d.Subdirectory != nil {
		opts.Subdirectory = d.Subdirectory
	}

	return opts
}

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

func IterForUpstream(upstream puller.Puller, delta UpstreamDelta) (UpstreamIter, error) {
	if delta.Subdirectory != nil {
		return nil, errors.New("incremental rebases do not support subdirectory changes")
	}

	switch u := upstream.(type) {
	case puller.GithubRepository:
		return NewGitIter(u.GetOptions(), delta)
	default:
		return NewSingleIter(upstream, delta)
	}
}

func NewSingleIter(upstream puller.Puller, delta UpstreamDelta) (UpstreamIter, error) {
	opts := delta.Apply(upstream.GetOptions())

	p, err := charts.GetUpstream(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get upstream: %w", err)
	}

	return &SingleIter{
		Upstream: p,
	}, nil
}
