package usermenu

import (
	"github.com/paranoidi/paras-commander/internal/entrymatch"
)

// EvalWhen evaluates a visibility expression; empty string is true.
func EvalWhen(expr string, ctx *EvalContext) (bool, error) {
	return entrymatch.EvalWhen(expr, ctx)
}

// EvalWhenAny evaluates multiple visibility expressions with OR semantics.
func EvalWhenAny(exprs []string, ctx *EvalContext) (bool, error) {
	return entrymatch.EvalWhenAny(exprs, ctx)
}
