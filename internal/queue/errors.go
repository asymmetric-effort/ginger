package queue

import "errors"

// ErrQueueClosed is returned when an operation is attempted on a closed queue.
var ErrQueueClosed = errors.New("queue is closed")
