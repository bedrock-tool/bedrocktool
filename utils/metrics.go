package utils

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
	"github.com/sirupsen/logrus"
)

const metricNamespace = "skin_bot"

type Metrics struct {
	Pusher           *push.Pusher
	RunningBots      prometheus.Gauge
	DisconnectEvents prometheus.Gauge
	DeadBots         prometheus.Gauge
}

func (m *Metrics) Attach(p *push.Pusher) {
	p.Collector(m.RunningBots).
		Collector(m.DisconnectEvents).
		Collector(m.DeadBots)
	m.Pusher = p
	p.Push()

	go func() {
		t := time.NewTicker(15 * time.Second)
		for range t.C {
			if err := m.Pusher.Add(); err != nil {
				logrus.Warnf("Failed to push metrics %s", err)
			}
		}
	}()
}

func NewMetrics() *Metrics {
	m := &Metrics{
		RunningBots: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Name:      "running_bots",
			Help:      "How many bots are currently running",
		}),
		DisconnectEvents: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Name:      "disconnect_events",
			Help:      "How many times this instance has had an disconnect event",
		}),
		DeadBots: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Name:      "dead_bots",
			Help:      "Count of bots in this instance that are dead and not working",
		}),
	}

	return m
}
