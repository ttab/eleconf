package eleconf

import "context"

type ChangeOp string

const (
	OpAdd    ChangeOp = "+"
	OpUpdate ChangeOp = "~"
	OpRemove ChangeOp = "-"
)

type ConfigurationChange interface {
	Describe() (ChangeOp, string)
	Execute(ctx context.Context, c Clients) error
}
