package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type MetricRegistry struct {
	Appname string          `json:"appname"`
	Node    string          `json:"node"`
	Metrics map[string]*int `json:"metrics"`
}

func NewMetricRegistry(appname string) *MetricRegistry {
	m := MetricRegistry{}
	m.Appname = appname
	m.Metrics = make(map[string]*int)
	host, _ := os.Hostname()
	m.Node = fmt.Sprintf("%s:%s:%d", appname, host, os.Getpid())
	return &m
}

func (m MetricRegistry) String() (s string) {
	b, err := json.Marshal(m)
	if err != nil {
		s = ""
		return
	}
	s = string(b)
	return
}

func (m MetricRegistry) NewCounter(name string) *int {
	c := 0
	m.Metrics[name] = &c
	return &c
}

// Incr, Decr, Get, Reset
func (m MetricRegistry) Incr(name string) {
	*(m.GetCounter(name))++
}

func (m MetricRegistry) Decr(name string) {
	*(m.GetCounter(name))--
}

func (m MetricRegistry) Get(name string) int {
	return *(m.GetCounter(name))
}

func (m MetricRegistry) Reset(name string) {
	*(m.GetCounter(name)) = 0
}

func (m MetricRegistry) GetCounter(name string) *int {
	if c := m.Metrics[name]; c != nil {
		return c
	} else {
		return m.NewCounter(name)
	}
}
