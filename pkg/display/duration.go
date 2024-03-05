package display

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// Its not important to be exact, approximations are good enough.
const (
	APPROXIMATE_HOURS_PER_DAY   = 24
	APPROXIMATE_HOURS_PER_WEEK  = APPROXIMATE_HOURS_PER_DAY * 7
	APPROXIMATE_HOURS_PER_MONTH = APPROXIMATE_HOURS_PER_DAY * 30
	APPROXIMATE_HOURS_PER_YEAR  = APPROXIMATE_HOURS_PER_DAY * 365
)

type Duration struct {
	Minutes int64
	Hours   int64
	Days    int64
	Weeks   int64
	Months  int64
	Years   int64
}

func NewDuration(d time.Duration) Duration {
	newD := Duration{
		Minutes: int64(d.Minutes()) % 60,
		Hours:   int64(d.Hours()) % 24,
		// Days:    int64(d.Hours()) % APPROXIMATE_HOURS_PER_WEEK / APPROXIMATE_HOURS_PER_DAY,
		Days:   int64(math.Round(float64(int64(d.Hours())%APPROXIMATE_HOURS_PER_WEEK) / float64(APPROXIMATE_HOURS_PER_DAY))),
		Weeks:  int64(math.Round(float64(int64(d.Hours())%APPROXIMATE_HOURS_PER_MONTH) / float64(APPROXIMATE_HOURS_PER_WEEK))),
		Months: int64(math.Round(float64(int64(d.Hours())%APPROXIMATE_HOURS_PER_YEAR) / float64(APPROXIMATE_HOURS_PER_MONTH))),
		// Months: int64(d.Hours()) % APPROXIMATE_HOURS_PER_YEAR / APPROXIMATE_HOURS_PER_MONTH,
		Years: int64(d.Hours()) / int64(APPROXIMATE_HOURS_PER_YEAR),
	}

	return newD
}

// Round returns a new duration rounded the duration to the highest non-zero field.
func (d Duration) Round() Duration {
	// if d.Minutes > 30 && d.Hours > 0 {
	// 	d.Hours++
	// 	d.Minutes = 0
	// }

	// if d.Hours > 12 && d.Days > 0 {
	// 	d.Days++
	// 	d.Hours = 0
	// }

	// if d.Days > 3 && d.Weeks > 0 {
	// 	d.Weeks++
	// 	d.Days = 0
	// }

	// if d.Weeks > 2 && d.Months > 0 {
	// 	d.Months++
	// 	d.Weeks = 0
	// }

	// if d.Months > 6 && d.Years > 0 {
	// 	d.Years++
	// 	d.Months = 0
	// }

	if d.Years > 0 {
		if d.Months >= 5 {
			return Duration{
				Years: d.Years + 1,
			}
		}

		return Duration{
			Years: d.Years,
		}
	}

	if d.Months > 0 {
		if d.Weeks >= 2 {
			return Duration{
				Months: d.Months + 1,
			}
		}

		return Duration{
			Months: d.Months,
		}
	}

	if d.Weeks > 0 {
		if d.Days > 3 {
			return Duration{
				Weeks: d.Weeks + 1,
			}
		}

		return Duration{
			Weeks: d.Weeks,
		}
	}

	if d.Days > 0 {
		if d.Hours >= 12 {
			return Duration{
				Days: d.Days + 1,
			}
		}

		return Duration{
			Days: d.Days,
		}
	}

	if d.Hours > 0 {
		if d.Minutes >= 30 {
			return Duration{
				Hours: d.Hours + 1,
			}
		}

		return Duration{
			Hours: d.Hours,
		}
	}

	return d
}

// StringifyDuration returns a human readable string of the duration.
func (d Duration) String() string {
	builder := strings.Builder{}

	if d.Years > 0 {
		builder.WriteString(fmt.Sprintf("%d year", d.Years))
		if d.Years > 1 {
			builder.WriteString("s")
		}
		builder.WriteRune(' ')
	}

	if d.Months > 0 {
		builder.WriteString(fmt.Sprintf("%d month", d.Months))
		if d.Months > 1 {
			builder.WriteString("s")
		}
		builder.WriteRune(' ')
	}

	if d.Weeks > 0 {
		builder.WriteString(fmt.Sprintf("%d week", d.Weeks))
		if d.Weeks > 1 {
			builder.WriteString("s")
		}
		builder.WriteRune(' ')
	}

	if d.Days > 0 {
		builder.WriteString(fmt.Sprintf("%d day", d.Days))
		if d.Days > 1 {
			builder.WriteString("s")
		}
		builder.WriteRune(' ')
	}

	if d.Hours > 0 {
		builder.WriteString(fmt.Sprintf("%d hour", d.Hours))
		if d.Hours > 1 {
			builder.WriteString("s")
		}
		builder.WriteRune(' ')
	}

	if d.Minutes > 0 {
		builder.WriteString(fmt.Sprintf("%d minute", d.Minutes))
		if d.Minutes > 1 {
			builder.WriteString("s")
		}
		builder.WriteRune(' ')
	}

	out := builder.String()
	out = strings.TrimSpace(out)

	return out
}
