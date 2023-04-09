package main

import (
	"fmt"
)

// TODO: Support env vars from https://developer.hashicorp.com/nomad/docs/commands#environment-variables
var (
	nomadAddr = getEnvDefault("NOMAD_ADDR", "http://localhost:4646")
)

func main() {
	m, err := New(nomadAddr)
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
