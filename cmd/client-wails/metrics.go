package main

import "sync/atomic"

type Metrics struct {
	BytesSent     uint64 `json:"bytesSent"`
	BytesReceived uint64 `json:"bytesReceived"`
}

var globalMetrics Metrics

func (a *App) GetMetrics() Metrics {
	return Metrics{
		BytesSent:     atomic.LoadUint64(&globalMetrics.BytesSent),
		BytesReceived: atomic.LoadUint64(&globalMetrics.BytesReceived),
	}
}

func addSent(n uint64) {
	atomic.AddUint64(&globalMetrics.BytesSent, n)
}

func addReceived(n uint64) {
	atomic.AddUint64(&globalMetrics.BytesReceived, n)
}
