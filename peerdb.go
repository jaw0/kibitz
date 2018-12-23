// Copyright (c) 2018
// Author: Jeff Weisberg <jaw @ tcp4me.com>
// Created: 2018-Nov-04 08:27 (EST)
// Function: peer db

package kibitz

import (
	"expvar"
	"sync"
	"time"

	"github.com/jaw0/kibitz/lamport"
)

const (
	KEEPDOWN = uint64(10 * time.Minute) // keep data about down servers for how long?
	KEEPLOST = uint64(10 * time.Minute) // keep data about servers we have not heard about for how long?
)

var serverupds = expvar.NewInt("kibitz_server_updates")

type infoer interface {
	Send(string, time.Duration, *PeerInfo) (bool, error)
	Notify(string, bool, bool)
}

type Conf struct {
	Iface       infoer
	System      string
	Seed        []string
	Id          string
	Hostname    string
	Environment string
	Datacenter  string
	Rack        string
	Promiscuous bool
	Port        int
}

type DB struct {
	iface       infoer
	sys         string
	id          string
	env         string
	dc          string
	rack        string
	host        string
	promiscuous bool // collect data on all system types?
	port        int  // tcp port
	seed        []string
	myaddrs     map[string]string
	mydoms      map[string]bool
	nmon        *netMon
	netinfo     []*NetInfo
	bestaddr    string
	stop        chan struct{}
	done        sync.WaitGroup
	clock       *lamport.Clock
	bootTime    uint64
	lock        sync.RWMutex
	allpeers    map[string]*Peer
	skeptical   map[string]*Peer
	kibitzers   map[string]*Peer
}

func New(c *Conf) *DB {

	pdb := &DB{
		iface:       c.Iface,
		sys:         c.System,
		env:         c.Environment,
		host:        c.Hostname,
		dc:          c.Datacenter,
		rack:        c.Rack,
		promiscuous: c.Promiscuous,
		port:        c.Port,
		seed:        c.Seed,
		clock:       lamport.New(),
		nmon:        netMonNew(),
		stop:        make(chan struct{}),
		myaddrs:     make(map[string]string),
		mydoms:      make(map[string]bool),
		allpeers:    make(map[string]*Peer),
		skeptical:   make(map[string]*Peer),
		kibitzers:   make(map[string]*Peer),
	}

	pdb.bootTime = pdb.clock.Now().Uint64()

	if pdb.env == "" {
		pdb.env = "dev"
	}

	pdb.learn(c)

	return pdb
}

func (pdb *DB) Start() {
	pdb.done.Add(1)
	go pdb.periodic()
}

func (pdb *DB) Stop() {
	close(pdb.stop)
	pdb.done.Wait()
}

// ################################################################

// 3rd party reports
func (pdb *DB) Update(px PeerImport) {

	pi := px.GetPeerInfo()

	dl.Debug("update peer %s", pi.GetServerId())

	if !pdb.isOK(pi) {
		return
	}

	serverupds.Add(1)

	pdb.lock.Lock()
	defer pdb.lock.Unlock()

	// update lamport clock
	pdb.clock.Update(lamport.ToTime(pi.GetTimeCreated()))
	pdb.clock.Update(lamport.ToTime(pi.GetTimeChecked()))

	p := pdb.find(pi.GetServerId())

	switch {

	case p == nil:
		dl.Verbose("discovered new peer %s", pi.GetServerId())
		p = peerNew(pdb, px, STATUS_UNKNOWN)
		pdb.allpeers[p.id] = p

	case p.status == STATUS_SCEPTICAL:
		dl.Verbose("discovered new peer %s", pi.GetServerId())
		pdb.upgrade(p)

	default:
		dl.Debug("update existing peer %s", pi.GetServerId())
	}

	// update status
	p.Update(px, pdb)
}

// their reports
func (pdb *DB) UpdateSceptical(px PeerImport) {

	pi := px.GetPeerInfo()

	if !pdb.isOK(pi) {
		return
	}

	serverupds.Add(1)

	pdb.lock.Lock()
	defer pdb.lock.Unlock()

	p := pdb.find(pi.GetServerId())

	if p == nil {
		dl.Debug("add new scept %s", pi.GetServerId())
		p = peerNew(pdb, px, STATUS_SCEPTICAL)
		pdb.skeptical[p.id] = p
	}
}

//################################################################

// our tests
func (pdb *DB) PeerUp(id string) {

	dl.Debug("up %s", id)

	pdb.lock.Lock()
	defer pdb.lock.Unlock()

	p := pdb.find(id)

	if p == nil {
		return
	}

	os := p.status

	if os == STATUS_SCEPTICAL {
		pdb.upgrade(p)
	}

	p.SetIsUp(pdb.clock.Inc())

	if os != STATUS_UP {
		dl.Debug("peer %s is now up", id)
	}
}

func (pdb *DB) PeerDn(id string) {

	dl.Debug("dn %s", id)

	pdb.lock.Lock()
	defer pdb.lock.Unlock()

	p := pdb.find(id)

	if p == nil {
		return
	}

	os := p.status

	if os == STATUS_SCEPTICAL {
		pdb.kill(p)
		return
	}

	p.SetMaybeDn(pdb.clock.Inc())

	if os != STATUS_DOWN && p.status == STATUS_DOWN {
		dl.Debug("peer %s is now down", id)
	}
}

// ################################################################

func (pdb *DB) isOK(p *PeerInfo) bool {

	now := pdb.clock.Now().Uint64()

	if p.GetServerId() == pdb.id {
		// NB - updates about ourself get discarded here
		return false
	}
	if p.GetSubsystem() != pdb.sys && !pdb.promiscuous {
		dl.Debug("not ok - sys - %v", p)
		return false
	}
	if p.GetEnvironment() != pdb.env {
		dl.Debug("not ok - env - %v", p)
		return false
	}

	if p.GetTimeCreated() < now-KEEPLOST {
		dl.Debug("not ok - Tchk - %v", p)
		return false
	}
	if p.GetTimeUp() < now-KEEPDOWN {
		dl.Debug("not ok - Tup - %v", p)
		return false
	}

	return true
}

// ################################################################

func (pdb *DB) find(id string) *Peer {

	p := pdb.allpeers[id]
	if p != nil {
		return p
	}

	p = pdb.skeptical[id]

	return p
}

func (pdb *DB) upgrade(p *Peer) {

	delete(pdb.skeptical, p.id)

	pdb.allpeers[p.id] = p
	if pdb.sys == p.info.GetSubsystem() {
		pdb.kibitzers[p.id] = p
	}
}

func (pdb *DB) kill(p *Peer) {

	delete(pdb.allpeers, p.id)
	delete(pdb.skeptical, p.id)
	delete(pdb.kibitzers, p.id)

	p.Kill()
}

// ################################################################

func (p *DB) Rack() string {
	return p.rack
}
func (p *DB) Datacenter() string {
	return p.dc
}
func (p *DB) Host() string {
	return p.host
}
func (p *DB) Env() string {
	return p.env
}
func (p *DB) Id() string {
	return p.id
}
func (p *DB) DomOK(dom string) bool {
	return p.mydoms[dom]
}
func (p *DB) ClockBoot() uint64 {
	return p.bootTime
}
func (p *DB) ClockNow() uint64 {
	return p.clock.Inc().Uint64()
}

func (pdb *DB) Get(id string) *Peer {
	pdb.lock.Lock()
	defer pdb.lock.Unlock()

	p := pdb.find(id)
	if p == nil {
		return nil
	}
	return p
}

// NB - does not include myself
func (pdb *DB) GetAll() []*Peer {

	var all []*Peer

	pdb.lock.RLock()
	defer pdb.lock.RUnlock()

	for _, p := range pdb.allpeers {
		all = append(all, p)
	}

	return all
}

// ################################################################

func (pdb *DB) periodic() {

	for {
		pdb.kibitzWithRandomPeer()
		pdb.Cleanup()

		delay := 5 * time.Second

		if len(pdb.kibitzers) == 0 || len(pdb.skeptical) != 0 {
			// faster at startup
			delay = time.Second
		}

		select {
		case <-pdb.stop:
			dl.Debug("done")
			pdb.done.Done()
			return
		case <-time.After(delay):
			continue
		}
	}
}

func (pdb *DB) Cleanup() {

	pdb.lock.Lock()
	defer pdb.lock.Unlock()

	// remove old entries
	for id, p := range pdb.allpeers {
		if !pdb.isOK(p.info) {
			dl.Debug("deleting %s", id)
			pdb.kill(p)
		}
	}
}
