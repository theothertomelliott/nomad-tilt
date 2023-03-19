package main

import (
	"fmt"
	"log"
	"os"
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

	jobSpec, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	job, err := c.Jobs().ParseHCL(string(jobSpec), false)
	jobID := *job.ID
	fmt.Println("Monitoring Job:", jobID)

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
