package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

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

	expectRunning := true
	for {
		liveJob, _, err := c.Jobs().Info(jobID, nil)
		if err != nil {
			panic(err)
		}

		status := *liveJob.Status
		if status == "dead" && expectRunning {
			log.Printf("Job is dead")
			expectRunning = false
		} else if !expectRunning {
			log.Printf("Job is %v", status)
			expectRunning = true
		}
		time.Sleep(5 * time.Second)
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
