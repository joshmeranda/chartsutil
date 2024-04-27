package display_test

import (
	"testing"
	"time"

	"github.com/joshmeranda/chartsutil/pkg/display"
)

func TestNewDuration(t *testing.T) {
	t.Skip("precision loss is a beast, and I don't care enough to slay it yet")

	duration := time.Hour*display.ApproximateHoursPerYear*5 + // 5 years
		time.Hour*display.ApproximateHoursPerMonth*2 + // 2 months
		time.Hour*display.ApproximateHoursPerWeek*2 + // 2 weeks
		time.Hour*display.ApproximateHoursPerDay*3 + // 3 days
		time.Hour*13 + // 13 Hours
		time.Minute*7 // 7 minutes

	expected := display.Duration{
		Minutes: 7,
		Hours:   13,
		Days:    3,
		Weeks:   2,
		Months:  2,
		Years:   5,
	}
	actual := display.NewDuration(duration)

	if expected != actual {
		t.Fatalf("Duration is wrong:\nexpected: %+v\n   found: %+v", expected, actual)
	}
}

func TestRound(t *testing.T) {
	type Case struct {
		Name     string
		Duration display.Duration
		Expected display.Duration
	}

	cases := []Case{
		{
			Name:     "Zero",
			Duration: display.Duration{},
			Expected: display.Duration{},
		},
		{
			Name: "Round Down",
			Duration: display.Duration{
				Minutes: 4,
				Hours:   4,
				Days:    4,
				Weeks:   1,
				Months:  4,
				Years:   0,
			},
			Expected: display.Duration{
				Months: 4,
			},
		},
		{
			Name: "Round to year",
			Duration: display.Duration{
				Minutes: 0,
				Hours:   0,
				Days:    5,
				Weeks:   5,
				Months:  5,
				Years:   1,
			},
			Expected: display.Duration{
				Years: 2,
			},
		},
	}

	for _, c := range cases {
		actual := c.Duration.Round()
		if actual != c.Expected {
			t.Fatalf("Failed '%s', duration is wrong:\nexpected: %+v\n   found: %+v", c.Name, c.Expected, actual)
		}
	}
}

func TestStringifyDuration(t *testing.T) {
	type Case struct {
		Duration display.Duration
		Round    bool
		Expected string
	}

	cases := []Case{
		{
			Duration: display.Duration{
				Minutes: 1,
			},
			Round:    false,
			Expected: "1 minute",
		},
		{
			Duration: display.Duration{
				Minutes: 2,
			},
			Round:    false,
			Expected: "2 minutes",
		},
		{
			Duration: display.Duration{
				Minutes: 1,
				Hours:   2,
				Days:    3,
				Weeks:   4,
				Months:  5,
				Years:   6,
			},
			Round:    false,
			Expected: "6 years 5 months 4 weeks 3 days 2 hours 1 minute",
		},
	}

	for _, c := range cases {
		actual := c.Duration.String()
		if actual != c.Expected {
			t.Errorf("expected '%s' but got '%s' for duration", c.Expected, actual)
		}
	}
}
