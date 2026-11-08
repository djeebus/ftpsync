package lib

type Queue[T any] struct {
	Elements []T
}

func (q *Queue[T]) Enqueue(elem T) {
	q.Elements = append(q.Elements, elem)
}

// Dequeue removes and returns the first element. It must not be called on an empty queue.
func (q *Queue[T]) Dequeue() T {
	element := q.Elements[0]
	q.Elements = q.Elements[1:]
	return element
}

func (q *Queue[T]) IsEmpty() bool {
	return len(q.Elements) == 0
}
