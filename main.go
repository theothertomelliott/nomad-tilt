package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/hashicorp/nomad/api"
)

func main() {
	c, err := api.NewClient(&api.Config{
		Address: "http://localhost:4646",
	})
	if err != nil {
		panic(err)
	}
	fmt.Println("Connected")
	defer c.Close()

	cancel := make(chan struct{})
	defer close(cancel)

	jobSpec, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	job, err := c.Jobs().ParseHCL(string(jobSpec), false)
	if err != nil {
		panic(err)
	}
	jobID := *job.ID
	fmt.Println("Monitoring Job:", jobID)

	events, err := c.EventStream().Stream(
		context.Background(),
		map[api.Topic][]string{
			api.TopicAllocation: {jobID},
		},
		0,
		nil,
	)
	if err != nil {
		panic(err)
	}

	allocs, _, err := c.Jobs().Allocations(jobID, true, nil)
	if err != nil {
		panic(err)
	}

	for _, alloc := range allocs {
		allocation, _, err := c.Allocations().Info(alloc.ID, nil)
		if err != nil {
			panic(err)
		}

		for task, taskState := range alloc.TaskStates {
			// Don't output logs for non-running tasks
			if taskState.State != "running" {
				continue
			}
			outputLogsForTask(c, allocation, task, "stdout", cancel)
			outputLogsForTask(c, allocation, task, "stderr", cancel)
		}
	}

	for ev := range events {
		for _, event := range ev.Events {
			if event.Type != "AllocationUpdated" {
				continue
			}
			alloc, err := event.Allocation()
			if err != nil {
				panic(err)
			}
			log.Printf("%v is %v", alloc.ID, alloc.ClientStatus)
		}
	}
}

func outputLogsForTask(c *api.Client, allocation *api.Allocation, task string, logType string, cancel <-chan struct{}) {
	logStream, errorStream := c.AllocFS().Logs(
		allocation,
		true,
		task,
		logType,
		"start",
		0,
		cancel,
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
