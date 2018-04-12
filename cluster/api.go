package cluster

import (
	"time"
)

var globalCluster *Cluster

func InitGlobalCluster(agents []string, contextExpirationTime time.Duration) error {
	c, err := NewCluster(agents, contextExpirationTime)
	if err != nil {
		return err
	}

	globalCluster = c
	return nil
}

func C() *Cluster {
	return globalCluster
}
