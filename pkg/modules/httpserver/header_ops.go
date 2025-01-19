package httpserver

import (
	"net/http"

	"go.n16f.net/bcl"
	"go.n16f.net/boulevard/pkg/boulevard"
	"go.n16f.net/program"
)

type HeaderOpType string

const (
	HeaderOpTypeSet    HeaderOpType = "set"
	HeaderOpTypeAdd    HeaderOpType = "add"
	HeaderOpTypeRemove HeaderOpType = "remove"
)

type HeaderOps []HeaderOp

type HeaderOp struct {
	Type  HeaderOpType
	Name  string
	Value *boulevard.FormatString
}

func (ops *HeaderOps) ReadBCLElement(block *bcl.Element) error {
	var ops2 HeaderOps

	readEntries := func(name string, t HeaderOpType) {
		for _, entry := range block.FindEntries(name) {
			var name string
			var value boulevard.FormatString

			entry.Values(&name, &value)

			ops2 = append(ops2, HeaderOp{
				Type:  t,
				Name:  name,
				Value: &value,
			})
		}
	}

	readEntries("set", HeaderOpTypeSet)
	readEntries("add", HeaderOpTypeAdd)
	readEntries("remove", HeaderOpTypeRemove)

	*ops = ops2
	return nil
}

func (ops HeaderOps) Apply(header http.Header, vars map[string]string) {
	for _, op := range ops {
		value := op.Value.Expand(vars)

		switch op.Type {
		case HeaderOpTypeSet:
			header.Set(op.Name, value)
		case HeaderOpTypeAdd:
			header.Add(op.Name, value)
		case HeaderOpTypeRemove:
			header.Del(op.Name)
		default:
			program.Panic("unhandled header operation %q", op.Type)
		}
	}
}
