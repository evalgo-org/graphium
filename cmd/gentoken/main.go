package main

import (
	"fmt"
	"time"
	"evalgo.org/graphium/internal/auth"
)

func main() {
	// Use the agent secret from config.yaml
	agentSecret := "change-me-in-production-use-different-secret-for-agents"
	hostID := "localhost-docker"
	expiration := 8760 * time.Hour // 1 year

	token, err := auth.GenerateAgentToken(agentSecret, hostID, expiration)
	if err != nil {
		panic(err)
	}

	fmt.Println(token)
}
