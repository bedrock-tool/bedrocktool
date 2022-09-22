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
	RunningBots      *prometheus.GaugeVec
	DisconnectEvents *prometheus.GaugeVec
	Deaths           *prometheus.GaugeVec
}

func (m *Metrics) Attach(p *push.Pusher) {
	p.Collector(m.RunningBots).
		Collector(m.DisconnectEvents).
		Collector(m.Deaths)
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
		RunningBots: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Name:      "running_bots",
			Help:      "How many bots are currently running",
		}, []string{"server"}),
		DisconnectEvents: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Name:      "disconnect_events",
			Help:      "How many times this instance has had an disconnect event",
		}, []string{"server", "ip"}),
		Deaths: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Name:      "bot_deaths",
			Help:      "how many times any bot has died",
		}, []string{"server", "ip"}),
	}

	return m
}
