package release_test

import (
	"testing"

	"github.com/joshmeranda/chartsutil/pkg/release"
)

func TestRoleRef(t *testing.T) {
	type Case struct {
		Name string
		In   string

		ExpectedRef release.RepoRef
		ExpectsErr  bool
	}

	cases := []Case{
		{
			Name: "Normal Upstream URL",
			In:   "https://github.com/joshmeranda/chartsutil.git",
			ExpectedRef: release.RepoRef{
				Owner: "joshmeranda",
				Name:  "chartsutil",
			},
		},

		{
			Name:       "Normal Upstream URL",
			In:         "https://github.com/chartsutil.git",
			ExpectsErr: true,
		},
	}

	for _, c := range cases {
		actualRef, err := release.RepoRefFromUrl(c.In)

		if c.ExpectsErr && err == nil {
			t.Fatalf("%s expected err but received None", c.Name)
		} else if !c.ExpectsErr && err != nil {
			t.Fatalf("%s does not expect err but recevied '%s'", c.Name, err)
		}

		if actualRef != c.ExpectedRef {
			t.Fatalf("%s recevied wrong ref:\nexpected: %+v\n  actual: %+v", c.Name, c.ExpectedRef, actualRef)
		}
	}
}
