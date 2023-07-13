package main

import (
	"context"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

/*
TODO
- [] Get network usage information from containers
- [] Use single dns server for all containers
- [] Store usage in database
- [] Start a VPN container
- [] Start a DNS container
- [] Accept HTTP CONNECT requests
- [] Region Selection Algorithm based on past bandwidth in that region for the requested domain
- [] VPN Server Selection based on current load and the heuristic mentioned above
- [] Slow Server Pruning
- [] Admin Panel
*/

var DOCKER_CLIENT *client.Client

func init() {
	os.Setenv("DOCKER_API_VERSION", "1.42")
	var err error
	DOCKER_CLIENT, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}

	_, err = DOCKER_CLIENT.Ping(context.Background())
	if err != nil {
		panic(err)
	}
}

func main() {
	const VPN_LABEL = "aa2.vpn"

	containers, err := DOCKER_CLIENT.ContainerList(context.Background(), types.ContainerListOptions{
		Filters: filters.NewArgs(filters.Arg("label", VPN_LABEL)),
	})
	if err != nil {
		panic(err)
	}

	if len(containers) == 0 {
		panic("No containers found")
	}

	containerSpeedChannels := make([]<-chan NetworkSpeed, len(containers))
	for i, container := range containers {
		containerStatChannel := make(chan NetworkStats)
		go pollContainerNetworkStats(container.ID, containerStatChannel, 5*time.Second)

		containerSpeedChannels[i] = calculateNetworkSpeed(containerStatChannel, 5*time.Second)
	}

	for {
		for _, channel := range containerSpeedChannels {
			speed := <-channel
			println(speed.String())
		}
	}

}
