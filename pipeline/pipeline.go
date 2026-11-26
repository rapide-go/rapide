package pipeline

import "slices"

// Pipe transforms or intercepts a passable value T before calling next.
type Pipe[T any] func(passable T, next func(T) (T, error)) (T, error)

// Pipeline passes a value of type T through a chain of stages.
type Pipeline[T any] struct {
	passable T
	pipes    []Pipe[T]
}

// Send initializes a generic Pipeline with the starting value.
func Send[T any](passable T) *Pipeline[T] {
	return &Pipeline[T]{passable: passable}
}

// Through registers the ordered sequence of pipes to execute.
func (p *Pipeline[T]) Through(pipes ...Pipe[T]) *Pipeline[T] {
	p.pipes = append(p.pipes, pipes...)
	return p
}

// Then executes the pipeline and passes the final value to destination.
func (p *Pipeline[T]) Then(destination func(T) (T, error)) (T, error) {
	next := destination
	for _, pipe := range slices.Backward(p.pipes) {

		curr := next
		next = func(val T) (T, error) {
			return pipe(val, curr)
		}
	}
	return next(p.passable)
}

// ThenReturn executes the pipeline and returns the resulting value directly.
func (p *Pipeline[T]) ThenReturn() (T, error) {
	return p.Then(func(v T) (T, error) { return v, nil })
}
