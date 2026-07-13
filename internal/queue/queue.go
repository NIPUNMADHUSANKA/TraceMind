package queue

// NewQueue returns the durable in-memory queue used by API and worker wiring.
func NewQueue() *ReliableQueue {
	return NewReliableQueue(QueueConfig{
		MaxAttempts: 3,
	})
}
