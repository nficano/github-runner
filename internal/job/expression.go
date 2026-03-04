package job

import (
	"context"
	"fmt"
)

// ExpressionEvaluator evaluates GitHub Actions ${{ }} expressions.
// This is a scaffold implementation. A full implementation would support:
//   - String interpolation: ${{ github.sha }}
//   - Function calls: success(), failure(), always(), cancelled()
//   - Comparison operators: ==, !=, <, >, <=, >=
//   - Logical operators: &&, ||, !
//   - Context access: github.*, env.*, secrets.*, steps.*.outputs.*
//   - Contains, startsWith, endsWith, format, join functions
type ExpressionEvaluator struct {
	jobCtx *Context
}

// NewExpressionEvaluator creates a new expression evaluator with the given
// job context for variable resolution.
func NewExpressionEvaluator(jobCtx *Context) *ExpressionEvaluator {
	return &ExpressionEvaluator{
		jobCtx: jobCtx,
	}
}

// Evaluate evaluates a GitHub Actions expression string and returns
// the result as a string. This is a scaffold that always returns the
// expression unchanged.
func (e *ExpressionEvaluator) Evaluate(_ context.Context, expr string) (string, error) {
	// Scaffold: a full implementation would parse and evaluate the expression
	// tree, resolving context references against e.jobCtx.
	return expr, fmt.Errorf("expression evaluation not implemented: %s", expr)
}

// EvaluateCondition evaluates a conditional expression (used in step "if" fields)
// and returns whether the step should execute. This is a scaffold that defaults
// to true (run the step).
func (e *ExpressionEvaluator) EvaluateCondition(_ context.Context, _ string) (bool, error) {
	// Scaffold: a full implementation would evaluate the condition and return
	// true/false. Built-in functions like success(), failure(), always(), and
	// cancelled() would check the job context's step results.
	return true, nil
}
