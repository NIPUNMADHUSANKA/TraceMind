package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type SQSQueue struct {
	client   *sqs.Client
	queueURL string
}

type QueueHealth struct {
	Available string `json:"available"`
	InFlight  string `json:"inFlight"`
	Delayed   string `json:"delayed"`
}

func NewSQSQueue(
	client *sqs.Client,
	queueURL string,
) *SQSQueue {
	return &SQSQueue{
		client:   client,
		queueURL: queueURL,
	}
}

func (q *SQSQueue) Enqueue(
	ctx context.Context,
	job IngestionJob,
) error {
	body, err := json.Marshal(job)
	if err != nil {
		return err
	}

	_, err = q.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(q.queueURL),
		MessageBody: aws.String(string(body)),
	})

	return err
}

func (q *SQSQueue) Dequeue(ctx context.Context) (*types.Message, error) {
	result, err := q.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(q.queueURL),
		MaxNumberOfMessages: 1,
		WaitTimeSeconds:     20,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to receive SQS message: %w", err)
	}

	if len(result.Messages) == 0 {
		return nil, ErrQueueEmpty
	}

	return &result.Messages[0], nil
}

func (q *SQSQueue) Ack(receipt string, ctx context.Context) error {
	_, err := q.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(q.queueURL),
		ReceiptHandle: aws.String(receipt),
	})

	if err != nil {
		return err
	} else {
		return nil
	}
}

func (q *SQSQueue) Nack(receipt string, ctx context.Context) error {

	_, err := q.client.ChangeMessageVisibility(
		ctx,
		&sqs.ChangeMessageVisibilityInput{
			QueueUrl:          aws.String(q.queueURL),
			ReceiptHandle:     aws.String(receipt),
			VisibilityTimeout: 30, // this is waiting time from first attempt fail to visible it again
		},
	)
	return err
}

func (q *SQSQueue) Health(ctx context.Context) (*QueueHealth, error) {

	output, err := q.client.GetQueueAttributes(
		ctx,
		&sqs.GetQueueAttributesInput{
			QueueUrl: aws.String(q.queueURL),
			AttributeNames: []types.QueueAttributeName{
				types.QueueAttributeNameApproximateNumberOfMessages,
				types.QueueAttributeNameApproximateNumberOfMessagesNotVisible,
				types.QueueAttributeNameApproximateNumberOfMessagesDelayed,
			},
		},
	)

	if err != nil {
		return nil, err
	}

	return &QueueHealth{
		Available: output.Attributes["ApproximateNumberOfMessages"],
		InFlight:  output.Attributes["ApproximateNumberOfMessagesNotVisible"],
		Delayed:   output.Attributes["ApproximateNumberOfMessagesDelayed"],
	}, nil

}
