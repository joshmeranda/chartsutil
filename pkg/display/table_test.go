package display_test

import (
	"fmt"
	"testing"

	"github.com/joshmeranda/chartsutil/pkg/display"
)

func TestTable(t *testing.T) {
	table := display.NewTable("Name", "Age", "Hash")
	table.AddRow("v1.0.0", "1h", "1234567890")
	table.AddRow("v1.0.0", "7d", "1234567890")
	table.AddRow("some-super-long-tag-v1.0.0", "5s", "123")

	expected := `Name                       Age Hash       
v1.0.0                     1h  1234567890 
v1.0.0                     7d  1234567890 
some-super-long-tag-v1.0.0 5s  123        `

	if expected != table.String() {
		fmt.Println("Expected:")
		fmt.Println(expected)
		fmt.Println("Actual:")
		fmt.Println(table)
		t.Fail()
	}
}
