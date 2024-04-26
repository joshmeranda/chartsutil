package puller

import (
	"errors"
	"fmt"
	"io"

	"github.com/rancher/charts-build-scripts/pkg/puller"
)

type ForEachFunc func(p puller.Puller) error

func ForEach(iter PullerIter, fn ForEachFunc) error {
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

type PullerIter interface {
	Next() (puller.Puller, error)
}

type singleIter struct {
	p puller.Puller
}

func (i *singleIter) Next() (puller.Puller, error) {
	if i.p == nil {
		return i.p, io.EOF
	}

	p := i.p
	i.p = nil

	return p, nil
}

type IterOptions struct {
	ToCommit *string
}

func IterForUpstream(upstream puller.Puller, opts IterOptions) (PullerIter, error) {
	switch u := upstream.(type) {
	case puller.GithubRepository:
		if opts.ToCommit == nil {
			return nil, fmt.Errorf("to commit is required for github upostreams")
		}

		return NewGitIter(u.GetOptions(), *opts.ToCommit)
	default:
		return &singleIter{
			p: u,
		}, nil
	}
}
