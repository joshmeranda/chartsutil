package display

import (
	"fmt"
	"strings"
)

type Column struct {
	Header string
	Width  int
	Data   []string
}

type Table []Column

func NewTable(headers ...string) Table {
	t := make(Table, len(headers))
	for i, h := range headers {
		t[i] = Column{Header: h, Width: len(h)}
	}

	return t
}

func (t Table) AddRow(data ...string) error {
	if len(data) != len(t) {
		return fmt.Errorf("data count (%d) does not match header count (%d)", len(data), len(t))
	}

	for i, d := range data {
		t[i].Data = append(t[i].Data, d)
		t[i].Width = max(t[i].Width, len(d))
	}

	return nil
}

func (t Table) String() string {
	builder := strings.Builder{}

	for _, c := range t {
		builder.WriteString(fmt.Sprintf("%-*s", c.Width+1, c.Header))
	}
	builder.WriteRune('\n')

	for i := 0; i < len(t[0].Data); i++ {
		for _, c := range t {
			builder.WriteString(fmt.Sprintf("%-*s", c.Width+1, c.Data[i]))
		}

		if i < len(t[0].Data)-1 {
			builder.WriteRune('\n')
		}
	}

	return builder.String()
}
