package main

import (
	"container/ring"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/docker/docker/api/types"
)

type NetworkStats struct {
	RXBytes uint64
	TXBytes uint64
}

const KB = 1024
const MB = 1024 * 1024
const GB = 1024 * MB

func pollContainerNetworkStats(containerID string, statsChannel chan<- NetworkStats, refreshRate time.Duration) {
	var prevStat *NetworkStats
	for {
		stats, err := DOCKER_CLIENT.ContainerStats(context.Background(), containerID, false)
		if err != nil {
			close(statsChannel)
		}

		var stat types.StatsJSON
		if err := json.NewDecoder(stats.Body).Decode(&stat); err != nil {
			close(statsChannel)
		}
		stats.Body.Close()

		rxBytes := uint64(0)
		txBytes := uint64(0)

		for _, network := range stat.Networks {
			rxBytes += network.RxBytes
			txBytes += network.TxBytes
		}

		if prevStat != nil && prevStat.RXBytes == rxBytes && prevStat.TXBytes == txBytes {
			continue
		}

		netStats := NetworkStats{
			RXBytes: rxBytes,
			TXBytes: txBytes,
		}

		statsChannel <- netStats
		prevStat = &netStats
		time.Sleep(refreshRate)
	}
}

type NetworkSpeed struct {
	RXPerSec float64
	TXPerSec float64
}

func (speed NetworkSpeed) String() string {
	return fmt.Sprintf("RX: %.5f/MBs, TX: %.5f/MBs", speed.RXPerSec/MB, speed.TXPerSec/MB)
}

type StatSample struct {
	Stats NetworkStats
	Time  time.Time
}

func calculateNetworkSpeed(statsChannel <-chan NetworkStats, refreshRate time.Duration) <-chan NetworkSpeed {
	speedChannel := make(chan NetworkSpeed)

	go func() {
		statBuffer := ring.New(9)

		for {
			stat := <-statsChannel
			fmt.Printf("%v\n", stat)

			totalRXDelta := float64(0)
			totalTXDelta := float64(0)
			numSamples := 0
			if statBuffer.Value != nil {
				prevStat := statBuffer.Value.(*StatSample)
				rxDelta := float64(stat.RXBytes - prevStat.Stats.RXBytes)
				txDelta := float64(stat.TXBytes - prevStat.Stats.TXBytes)

				totalRXDelta += rxDelta
				totalTXDelta += txDelta
				numSamples++
			}

			var prevStat *StatSample
			statBuffer.Do(func(value interface{}) {
				if value == nil {
					return
				}

				if prevStat == nil {
					prevStat = value.(*StatSample)
					return
				}

				fmt.Printf("Bufferd: %v\n", value)

				rxDelta := float64(value.(*StatSample).Stats.RXBytes - prevStat.Stats.RXBytes)
				txDelta := float64(value.(*StatSample).Stats.TXBytes - prevStat.Stats.TXBytes)

				totalRXDelta += rxDelta
				totalTXDelta += txDelta
				numSamples++
			})

			speedChannel <- NetworkSpeed{
				RXPerSec: totalRXDelta / float64(numSamples) / refreshRate.Seconds(),
				TXPerSec: totalTXDelta / float64(numSamples) / refreshRate.Seconds(),
			}

			statBuffer.Value = &StatSample{
				Stats: stat,
				Time:  time.Now(),
			}

			statBuffer = statBuffer.Next()
		}
	}()

	return speedChannel
}
