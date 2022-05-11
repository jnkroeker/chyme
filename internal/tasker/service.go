package tasker

import (
	"fmt"

	"kroekerlabs.dev/chyme/services/internal/core"
)

type Service struct {
	ResourceSetKey     string
	ResourceRepository core.ResourceRepository
	TaskRepository     core.TaskRepository
	TaskQueue          core.TaskQueue
	Templater          Templater
	BatchSize          int
}

func New(rsKey string, rr core.ResourceRepository, tr core.TaskRepository, q core.TaskQueue, t Templater, batchSize int) Service {
	return Service{
		rsKey,
		rr,
		tr,
		q,
		t,
		batchSize,
	}
}

// Creates `count` tasks using the key repository as the source
// and the task queue as the task destination
func (s Service) CreateTasks(count int) (int, error) {
	sources, err := s.ResourceRepository.Pop(s.ResourceSetKey, count)
	fmt.Println("sources for tasks")
	fmt.Println(sources)
	if err != nil {
		return 0, err
	}

	// In the loop below we shift resources out of the source slice once they are successfully processed. This defer
	// will add any resources that have not been shifted off (i.e. failed processing) back to the set for another
	// attempt at processing.
	defer func() {
		if len(sources) > 0 {
			_, _ = s.ResourceRepository.Add(s.ResourceSetKey, sources...)
		}
	}()

	created := 0
	for len(sources) > 0 {
		source := sources[0]
		tasks := s.Templater.Create(source)
		c, err := s.enqueueTasks(tasks, false)
		if err != nil {
			return created, err
		}
		created += c
		sources = sources[1:]
	}

	return created, nil
}

func (s Service) ShouldCreate() (int, error) {
	// messageCount, err := s.TaskQueue.MessageCount()
	// if err != nil {
	// 	return 0, err
	// }

	// s.mcSMALock.Lock()
	// s.messageCountSMA.Add(float32(messageCount))
	// s.mcSMALock.Unlock()

	// if s.messageCountSMA.Avg() > float32(s.CreationThreshold) {
	// 	return 0, nil
	// }

	return s.BatchSize, nil
}

func (s Service) Poll() error {
	fmt.Println("poll")
	count, err := s.ShouldCreate()
	if err != nil {
		return err
	}

	_, err = s.CreateTasks(count)
	if err != nil {
		return err
	}

	return nil
}

func (s *Service) enqueueTasks(tasks []*core.Task, shouldDeduplicate bool) (int, error) {
	created := 0

	for _, task := range tasks {
		// if shouldDeduplicate {
		// 	exists, err := s.TaskRepository.Has(task)
		// 	if err != nil {
		// 		return created, err
		// 	}
		// 	if exists {
		// 		continue
		// 	}
		// }

		queue := s.TaskQueue
		// if task.QueueAffinity != "" {
		// 	q, ok := s.AlternateQueues.Get(task.QueueAffinity)
		// 	if !ok {
		// 		return created, errors.New("unknown queue " + task.QueueAffinity)
		// 	}
		// 	queue = q
		// }
		if err := queue.Enqueue(task); err != nil {
			return created, err
		}
		if err := s.TaskRepository.Add(task); err != nil {
			return created, err
		}
		created++
	}

	return created, nil
}

// func (s *service) MessageCountSMA() float32 {
// 	s.mcSMALock.RLock()
// 	defer s.mcSMALock.RUnlock()
// 	return s.messageCountSMA.Avg()
// }
