package table

import (
	"fmt"
	"io"
	"os"

	"text/tabwriter"

	"github.com/thirdeyenick/kubectl-free/pkg/util"
)

// OutputTable is struct of tables for outputs
type OutputTable struct {
	Header []string
	Rows   []string
	Output io.Writer
}

// NewOutputTable is an instance of OutputTable
func NewOutputTable(o io.Writer) *OutputTable {
	return &OutputTable{
		Output: o,
	}
}

// Print shows table output
func (t *OutputTable) Print() {

	// get printer
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)

	// write header
	if len(t.Header) > 0 {
		fmt.Fprintln(w, util.JoinTab(t.Header))
	}

	// write rows
	for _, row := range t.Rows {
		fmt.Fprintln(w, row)
	}

	// finish
	w.Flush()
}

// AddRow adds row to table
func (t *OutputTable) AddRow(s []string) {
	t.Rows = append(t.Rows, util.JoinTab(s))
}
