package queue

import "tracemind/internal/models"

type IngestionJob struct {
	IngestionID string
	Signals     []models.Signal
}

func NewQueue(size int) chan IngestionJob {
	return make(chan IngestionJob, size)
}
