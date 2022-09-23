package main

import (
	"encoding/hex"
	"math/rand"
	"os"
	"strings"
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

func (m *Metrics) Delete() {
	m.Pusher.Delete()
}

func (m *Metrics) Start(url, user, password string) error {
	m.Pusher = push.New(url, metricNamespace).
		BasicAuth(user, password).
		Grouping("node_id", getNodeId()).
		Collector(m.RunningBots).
		Collector(m.DisconnectEvents).
		Collector(m.Deaths)
	if err := m.Pusher.Push(); err != nil {
		return err
	}

	go func() {
		t := time.NewTicker(15 * time.Second)
		for range t.C {
			if err := m.Pusher.Add(); err != nil {
				logrus.Warnf("Failed to push metrics %s", err)
			}
		}
	}()

	return nil
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

func randomHex(n int) string {
	bytes := make([]byte, n)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func getNodeId() string {
	if _, err := os.Stat("node_id.txt"); err == nil {
		d, _ := os.ReadFile("node_id.txt")
		return strings.Split(string(d), "\n")[0]
	}

	ret := randomHex(10)
	os.WriteFile("node_id.txt", []byte(ret), 0o777)
	return ret
}
