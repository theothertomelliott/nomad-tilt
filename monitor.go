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

type Monitor struct {
	startTime time.Time
	client    *api.Client
	jobID     string

	cancel chan struct{}

	knownAllocations      map[string]struct{}
	allocationLogChannels map[string]chan struct{}

	events <-chan *api.Events
}

func New(addr string) (*Monitor, error) {
	var err error
	m := &Monitor{
		startTime:             time.Now(),
		knownAllocations:      make(map[string]struct{}),
		allocationLogChannels: make(map[string]chan struct{}),
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
		m.allocationLogChannels[allocation.ID] = make(chan struct{})

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
		m.allocationLogChannels[allocation.ID],
		nil,
	)
	go func() {
		allocationId := allocation.ID
		defer log.Printf("Allocation logs (%v) ended: %v", logType, allocationId)
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
				// TODO: Try to recover from this error
				log.Printf("Could not read logs for %v/%v (%v): %v", allocationId, task, logType, err)
				return
			case <-m.allocationLogChannels[allocationId]:
				return
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
		if alloc.ClientStatus == "complete" || alloc.ClientStatus == "failed" {
			func() {
				defer func() {
					if r := recover(); r != nil {
						fmt.Println("Recovered in f", r)
					}
				}()
				log.Printf("closing channel for %v", alloc.ID)
				close(m.allocationLogChannels[alloc.ID])
			}()
		}
	}
	return nil
}
