package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
)

func main() {
	m, err := New("http://localhost:4646")
	if err != nil {
		panic(err)
	}

	fmt.Println("Monitoring Job:", m.jobID)

	err = m.Start()
	if err != nil {
		panic(err)
	}

	for ev := range m.events {
		err := m.processEvent(ev)
		if err != nil {
			panic(err)
		}
	}
}

type Monitor struct {
	startTime time.Time
	client    *api.Client
	jobID     string

	cancel chan struct{}

	knownAllocations map[string]struct{}

	events <-chan *api.Events
}

func New(addr string) (*Monitor, error) {
	var err error
	m := &Monitor{
		startTime:        time.Now(),
		knownAllocations: make(map[string]struct{}),
	}

	m.client, err = api.NewClient(&api.Config{
		Address: addr,
	})
	if err != nil {
		return nil, err
	}

	jobSpec, err := os.ReadFile(os.Args[1])
	if err != nil {
		return nil, err
	}
	job, err := m.client.Jobs().ParseHCL(string(jobSpec), false)
	if err != nil {
		return nil, err
	}
	m.jobID = *job.ID

	m.cancel = make(chan struct{})
	return m, nil
}

func (m *Monitor) Start() error {
	var err error
	m.events, err = m.client.EventStream().Stream(
		context.Background(),
		map[api.Topic][]string{
			api.TopicAllocation: {"*"},
		},
		0,
		nil,
	)
	if err != nil {
		return err
	}

	allocs, _, err := m.client.Jobs().Allocations(m.jobID, true, nil)
	if err != nil {
		return err
	}

	for _, alloc := range allocs {
		m.watchAllocation(alloc.ID, true)
	}

	return nil
}

func (m *Monitor) watchAllocation(allocID string, checkTaskState bool) {
	allocation, _, err := m.client.Allocations().Info(allocID, nil)
	if err != nil {
		panic(err)
	}

	for task, taskState := range allocation.TaskStates {
		// Don't output logs for non-running tasks
		if checkTaskState && taskState.State != "running" {
			continue
		}

		// Record this allocation as one we're watching
		m.knownAllocations[allocation.ID] = struct{}{}

		m.outputLogsForTask(allocation, task, "stdout")
		m.outputLogsForTask(allocation, task, "stderr")
	}
}

func (m *Monitor) outputLogsForTask(allocation *api.Allocation, task string, logType string) {
	logStream, errorStream := m.client.AllocFS().Logs(
		allocation,
		true,
		task,
		logType,
		"start",
		0,
		m.cancel,
		nil,
	)
	go func() {
		allocationId := allocation.ID
		task := task
		logType := logType
		for {
			select {
			case logStream := <-logStream:
				if logStream != nil {
					lines := string(logStream.Data)
					lines = strings.TrimRight(lines, "\n")
					for _, line := range strings.Split(lines, "\n") {
						log.Printf("%v/%v (%v): %v", allocationId, task, logType, line)
					}
				}
			case err := <-errorStream:
				panic(err)
			}
		}
	}()
}

func (m *Monitor) processEvent(ev *api.Events) error {
	for _, event := range ev.Events {
		if event.Type != "AllocationUpdated" {
			continue
		}

		// Match either the job name or subjobs for the batch scheduler
		match := false
		for _, k := range event.FilterKeys {
			if k == m.jobID || strings.HasPrefix(k, fmt.Sprintf("%v/", m.jobID)) {
				match = true
			}
		}
		if !match {
			continue
		}

		alloc, err := event.Allocation()
		if err != nil {
			return err
		}
		if alloc == nil {
			continue
		}

		_, allocIsKnown := m.knownAllocations[alloc.ID]
		createTime := time.Unix(0, alloc.CreateTime)
		diff := createTime.Sub(m.startTime)
		if !allocIsKnown && diff < 0 {
			continue
		}
		log.Printf("Allocation %v is %v", alloc.ID, alloc.ClientStatus)
		if !allocIsKnown {
			m.watchAllocation(alloc.ID, false)
		}
	}
	return nil
}
