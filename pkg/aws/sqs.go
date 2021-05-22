package aws

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
)

const (
	MaxLongPollWaitSeconds     = 20
	MaxVisbilityTimeoutSeconds = 43200
)

type SqsQueue struct {
	Name string
	svc *sqs.SQS 
	url string
}

// Represents an object that has a handle that identifies it to an external service.
type Handled interface {
	Handle() interface{}
}

func NewSQSQueue(svc *sqs.SQS, queueName string) (*SqsQueue, error) {
	result, err := svc.GetQueueUrl(&sqs.GetQueueUrlInput{
		QueueName: &queueName,
	})
	if err != nil {
		return nil, err
	}

	return &SqsQueue{
		Name: queueName,
		svc: svc,
		url: *result.QueueUrl,
	}, nil
}

// Creates and sends a message to the SQS queue
func (q *SqsQueue) Enqueue(item interface{}) error {
	marshaled, err := json.Marshal(item)
	if err != nil {
		return err 
	}

	_, err = q.svc.SendMessage(&sqs.SendMessageInput{
		MessageBody: aws.String(string(marshaled)),
		QueueUrl:    &q.url,
	})
	return err 
}

func (q *SqsQueue) EnqueueWithAttrs(item interface{}, attrs map[string]*sqs.MessageAttributeValue) error {
	marshaled, err := json.Marshal(item)
	if err != nil {
		return err
	}

	_, err = q.svc.SendMessage(&sqs.SendMessageInput{
		MessageBody:       aws.String(string(marshaled)),
		MessageAttributes: attrs,
		QueueUrl:          &q.url,
	})
	return err
}

const SQSMaxMessagesPerReceive = 10

// Receives messages from the SQS queue.
func (q *SqsQueue) Dequeue(max int) ([]*sqs.Message, error) {
	if max < 1 || max > SQSMaxMessagesPerReceive {
		return nil, fmt.Errorf("max messages to dequeue out of range, must be between 1 and %d inclusive", SQSMaxMessagesPerReceive)
	}

	result, err := q.svc.ReceiveMessage(&sqs.ReceiveMessageInput{
		QueueUrl:              &q.url,
		WaitTimeSeconds:       aws.Int64(MaxLongPollWaitSeconds),
		MaxNumberOfMessages:   aws.Int64(int64(max)),
		MessageAttributeNames: []*string{aws.String("All")},
	})
	if err != nil {
		return nil, err
	}

	return result.Messages, nil
}

// Deletes a message from the SQS queue.
func (q *SqsQueue) Delete(h Handled) error {
	handle, ok := h.Handle().(*string)
	if !ok {
		s, ok := h.Handle().(string)
		if !ok {
			return errors.New("handle must be of type string or *string")
		}
		handle = &s 
	}
	_, err := q.svc.DeleteMessage(&sqs.DeleteMessageInput{
		QueueUrl:      &q.url,
		ReceiptHandle: handle,
	})
	return err 
}

const SQSMessageCountAttribute = "ApproximateNumberOfMessages"

func (q *SqsQueue) MessageCount() (int, error) {
	res, err := q.svc.GetQueueAttributes(&sqs.GetQueueAttributesInput{
		QueueUrl:       &q.url,
		AttributeNames: []*string{aws.String(SQSMessageCountAttribute)},
	})
	if err != nil {
		return 0, err
	}

	countStr, ok := res.Attributes[SQSMessageCountAttribute]
	if !ok {
		return 0, errors.New("SQS response Attributes map does not contain " + SQSMessageCountAttribute)
	}
	count, err := strconv.Atoi(*countStr)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// DequeueAll dequeues all messages in the queue and writes them to the returned channel.
func (q *SqsQueue) DequeueAll() (<-chan *sqs.Message, <-chan error) {
	msgCh := make(chan *sqs.Message)
	errCh := make(chan error, 1)

	go func() {
		isDrained := false

		for !isDrained {
			messages, err := q.Dequeue(SQSMaxMessagesPerReceive)
			if err != nil {
				errCh <- err
				break
			}

			if len(messages) > 0 {
				for _, msg := range messages {
					msgCh <- msg
				}
			} else {
				isDrained = true
			}
		}

		close(msgCh)
		close(errCh)
	}()

	return msgCh, errCh
}

