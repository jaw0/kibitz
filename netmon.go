// Copyright (c) 2018
// Author: Jeff Weisberg <jaw @ tcp4me.com>
// Created: 2018-Nov-10 13:46 (EDT)
// Function: track status of all of the networks we are connected to

package kibitz

import (
	"sync"
	"time"
)

const STALE = int64(2 * time.Minute)

type netMon struct {
	lock   sync.RWMutex
	lastUp map[string]int64
}

func netMonNew() *netMon {
	return &netMon{
		lastUp: make(map[string]int64),
	}
}

func (nm *netMon) Add(net string) {
	net = netName(net)
	dl.Debug("net + %s", net)

	nm.lock.Lock()
	defer nm.lock.Unlock()
	nm.lastUp[net] = now()
}

func (nm *netMon) SetUp(net string) {
	net = netName(net)

	nm.lock.Lock()
	defer nm.lock.Unlock()

	_, ok := nm.lastUp[net]
	if ok {
		nm.lastUp[net] = now()
	}
}

func (nm *netMon) IsUp(net string) (bool, bool) {
	net = netName(net)

	nm.lock.RLock()
	defer nm.lock.RUnlock()

	t, ok := nm.lastUp[net]
	if ok {
		return t >= now()-STALE, true
	}
	return false, false
}

func netName(n string) string {
	if n == "" {
		return "public"
	}
	return n
}

func now() int64 {
	return time.Now().UnixNano()
}
