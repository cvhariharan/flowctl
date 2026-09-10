package expreval

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

const (
	strictMaxNodes       = 100
	strictMaxOutput      = 4096
	strictMaxSource      = 512
	strictMaxMemory uint = 1000
	strictTimeout        = 500 * time.Millisecond
)

var templateRe = regexp.MustCompile(`(?s){{(.*?)}}`)

type Evaluator interface {
	HasTemplate(s string) bool
	ValidateTemplate(s string, env map[string]any) error
	Eval(expression string, env map[string]any) (any, error)
	EvalTemplate(s string, env map[string]any) (any, error)
	Interpolate(s string, env map[string]any) (string, error)
}

type Option func(*evaluator)

func Strict() Option {
	return func(e *evaluator) { e.strict = true }
}

type evaluator struct {
	strict bool
}

func New(opts ...Option) Evaluator {
	e := &evaluator{}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func (e *evaluator) HasTemplate(s string) bool {
	return templateRe.MatchString(s)
}

func normalizeEnv(env map[string]any) map[string]any {
	if env == nil {
		return map[string]any{}
	}
	return env
}

func (e *evaluator) checkSource(s string) error {
	if e.strict && len(s) > strictMaxSource {
		return fmt.Errorf("expression exceeds %d bytes", strictMaxSource)
	}
	return nil
}

func (e *evaluator) compile(expression string, env map[string]any) (*vm.Program, error) {
	opts := []expr.Option{expr.Env(normalizeEnv(env))}
	if e.strict {
		opts = append(opts, expr.MaxNodes(strictMaxNodes))
	}

	program, err := expr.Compile(expression, opts...)
	if err != nil {
		return nil, fmt.Errorf("could not compile expression %q: %w", expression, err)
	}
	return program, nil
}

func withDeadline[T any](fn func() (T, error)) (T, error) {
	type result struct {
		out T
		err error
	}

	ch := make(chan result, 1)
	go func() {
		out, err := fn()
		ch <- result{out: out, err: err}
	}()

	select {
	case r := <-ch:
		return r.out, r.err
	case <-time.After(strictTimeout):
		var zero T
		return zero, fmt.Errorf("evaluation exceeded %s", strictTimeout)
	}
}

func (e *evaluator) eval(expression string, env map[string]any) (any, error) {
	program, err := e.compile(expression, env)
	if err != nil {
		return nil, err
	}

	if e.strict {
		machine := vm.VM{MemoryBudget: strictMaxMemory}
		out, err := machine.Run(program, normalizeEnv(env))
		if err != nil {
			return nil, fmt.Errorf("could not evaluate expression %q: %w", expression, err)
		}
		return out, nil
	}

	out, err := expr.Run(program, normalizeEnv(env))
	if err != nil {
		return nil, fmt.Errorf("could not evaluate expression %q: %w", expression, err)
	}
	return out, nil
}

func (e *evaluator) Eval(expression string, env map[string]any) (any, error) {
	if err := e.checkSource(expression); err != nil {
		return nil, err
	}
	if !e.strict {
		return e.eval(expression, env)
	}
	return withDeadline(func() (any, error) { return e.eval(expression, env) })
}

func (e *evaluator) ValidateTemplate(s string, env map[string]any) error {
	if err := e.checkSource(s); err != nil {
		return err
	}

	for _, match := range templateRe.FindAllStringSubmatch(s, -1) {
		expression := strings.TrimSpace(match[1])
		if _, err := e.compile(expression, env); err != nil {
			return err
		}
	}
	return nil
}

func (e *evaluator) EvalTemplate(s string, env map[string]any) (any, error) {
	if err := e.checkSource(s); err != nil {
		return nil, err
	}

	match := templateRe.FindStringSubmatch(s)
	if match == nil {
		return nil, nil
	}

	expression := strings.TrimSpace(match[1])
	if !e.strict {
		return e.eval(expression, env)
	}
	return withDeadline(func() (any, error) { return e.eval(expression, env) })
}

func (e *evaluator) Interpolate(s string, env map[string]any) (string, error) {
	if err := e.checkSource(s); err != nil {
		return "", err
	}
	if !e.strict {
		return e.interpolate(s, env)
	}
	return withDeadline(func() (string, error) { return e.interpolate(s, env) })
}

func (e *evaluator) interpolate(s string, env map[string]any) (string, error) {
	var evalErr error

	out := templateRe.ReplaceAllStringFunc(s, func(match string) string {
		if evalErr != nil {
			return ""
		}

		sub := templateRe.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}

		value, err := e.eval(strings.TrimSpace(sub[1]), env)
		if err != nil {
			evalErr = err
			return ""
		}
		if value == nil {
			return ""
		}
		return fmt.Sprint(value)
	})

	if evalErr != nil {
		return "", evalErr
	}
	if e.strict && len(out) > strictMaxOutput {
		return "", fmt.Errorf("interpolated value exceeds %d bytes", strictMaxOutput)
	}
	return out, nil
}
