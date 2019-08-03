// Copyright (c) 2018
// Author: Jeff Weisberg <jaw @ tcp4me.com>
// Created: 2018-Nov-04 08:41 (EST)
// Function: peers

package kibitz

import (
	"sync"
	"time"

	"github.com/jaw0/kibitz/lamport"
)

type PeerStatus int

const (
	STATUS_UNKNOWN   PeerStatus = 0
	STATUS_UP        PeerStatus = 2
	STATUS_MAYBEDN   PeerStatus = 3
	STATUS_DOWN      PeerStatus = 4
	STATUS_SCEPTICAL PeerStatus = 5
	STATUS_DEAD      PeerStatus = 6
)

const (
	MAXFAIL = 3
	MAXVIA  = 1024
)

type PeerImport interface {
	GetPeerInfo() *PeerInfo
	SetPeerInfo(*PeerInfo)
}

type Peer struct {
	pdb      *DB
	lock     sync.Mutex
	id       string
	status   PeerStatus
	numFail  int
	lastTry  time.Time
	bestAddr string
	info     *PeerInfo
	data     PeerImport
}

type Export struct {
	Netinfo     []*NetInfo
	Status      PeerStatus
	Id          string
	Sys         string
	Env         string
	Hostname    string
	Rack        string
	Datacenter  string
	BestAddr    string
	TimeLastUp  uint64
	TimeUpSince uint64
	LastTry     time.Time
	IsUp        bool
	IsSameRack  bool
	IsSameDC    bool
}

func peerNew(pdb *DB, px PeerImport, st PeerStatus) *Peer {

	pi := px.GetPeerInfo()

	return &Peer{
		pdb:    pdb,
		status: st,
		data:   px,
		info:   pi,
		id:     pi.GetServerId(),
	}
}

// recvd updates
func (p *Peer) Update(px PeerImport, pdb *DB) {

	pi := px.GetPeerInfo()

	p.lock.Lock()
	defer p.lock.Unlock()

	switch {
	case pi.GetTimeCreated() <= p.info.GetTimeCreated():
		// discard old outdated update
		return

	case pi.GetTimeCreated() > p.info.GetTimeCreated():
	case pi.GetTimeChecked() > p.info.GetTimeChecked():
	case p.status == STATUS_UNKNOWN:
		break

	default:
		return
	}

	// did config change?
	changed := pi.GetTimeConf() > p.info.GetTimeConf()

	bestaddr := p.figureBestAddr(pi)
	if bestaddr != p.bestAddr {
		p.bestAddr = bestaddr
		changed = true
	}

	p.info = pi
	p.data = px

	via := pi.GetVia() + " " + pdb.id
	if len(via) > MAXVIA {
		via = via[:MAXVIA]
	}
	pi.Via = via

	// trap any invalid access
	px.SetPeerInfo(nil)

	p.changeStatus(PeerStatus(pi.GetStatusCode()), changed)

}

// results of our tests
func (p *Peer) SetIsUp(now lamport.Time) {

	p.lock.Lock()
	defer p.lock.Unlock()

	p.numFail = 0
	p.lastTry = time.Now()

	t := now.Uint64()
	p.info.TimeLastUp = t
	p.info.TimeChecked = t

	if p.status != STATUS_UP || p.info.TimeUpSince == 0 {
		p.info.TimeUpSince = t
	}

	p.changeStatus(STATUS_UP, false)
}

func (p *Peer) SetMaybeDn(now lamport.Time) {

	p.lock.Lock()
	defer p.lock.Unlock()

	p.numFail++
	p.lastTry = time.Now()

	t := now.Uint64()
	p.info.TimeChecked = t
	p.info.TimeUpSince = t

	if p.numFail > MAXFAIL || p.status == STATUS_DOWN {
		p.changeStatus(STATUS_DOWN, false)
		return
	}

	p.changeStatus(STATUS_MAYBEDN, false)
}

func (p *Peer) Kill() {

	p.lock.Lock()
	defer p.lock.Unlock()
	p.changeStatus(STATUS_DEAD, false)
}

// ################################################################

func (p *Peer) changeStatus(st PeerStatus, changed bool) bool {

	os := p.status

	p.status = st

	switch st {
	case STATUS_UP, STATUS_DOWN:
		p.info.SetStatusCode(st)
	}

	if os != st {
		dl.Debug("peer %s changed to %s", p.id, st)
	}

	if os == st && !changed {
		return false
	}

	switch st {
	case STATUS_UP, STATUS_DOWN, STATUS_DEAD:
		go p.pdb.iface.Notify(p.id, st == STATUS_UP, p.info.GetSubsystem() == p.pdb.sys)
		return true
	}

	return false
}

func (p *Peer) figureBestAddr(pi *PeerInfo) string {

	var best string

	for _, ni := range pi.NetInfo {
		dom := ni.GetNatdom()

		if dom != "" {
			if p.pdb.DomOK(dom) {
				// prefer reachable private network
				best = ni.GetAddr()
			}
		} else {
			if best == "" {
				// otherwise a public network
				best = ni.GetAddr()
			}
		}
	}

	return best
}

// ################################################################

func (p *Peer) GetData() interface{} {
	p.lock.Lock()
	defer p.lock.Unlock()

	// make and attach a copy of the PeerInfo
	pi := *p.info
	p.data.SetPeerInfo(&pi)

	return p.data
}

func (p *Peer) GetExport() *Export {

	p.lock.Lock()
	defer p.lock.Unlock()

	pi := p.info

	return &Export{
		Id:          p.id,
		Status:      p.status,
		Netinfo:     pi.GetNetInfo(),
		Sys:         pi.GetSubsystem(),
		Hostname:    pi.GetHostname(),
		Env:         pi.GetEnvironment(),
		Rack:        pi.GetRack(),
		Datacenter:  pi.GetDatacenter(),
		IsUp:        (pi.GetStatusCode() == int32(STATUS_UP)),
		BestAddr:    p.bestAddr,
		TimeLastUp:  pi.GetTimeLastUp(),
		TimeUpSince: pi.GetTimeUpSince(),
		LastTry:     p.lastTry,
		IsSameRack:  (pi.GetRack() == p.pdb.rack),
		IsSameDC:    (pi.GetDatacenter() == p.pdb.dc),
	}
}

func (pdb *DB) GetExportSelf() *Export {

	now := pdb.clock.Inc().Uint64()

	return &Export{
		Id:          pdb.id,
		Status:      STATUS_UP,
		Env:         pdb.env,
		Sys:         pdb.sys,
		Netinfo:     pdb.netinfo,
		Hostname:    pdb.host,
		Rack:        pdb.rack,
		Datacenter:  pdb.dc,
		IsUp:        true,
		BestAddr:    pdb.bestaddr,
		TimeLastUp:  now,
		TimeUpSince: now,
		IsSameRack:  true,
		IsSameDC:    true,
	}
}

func (p *Peer) getAddrs() []*NetInfo {
	p.lock.Lock()
	defer p.lock.Unlock()

	return p.info.NetInfo
}

func (s PeerStatus) String() string {
	switch s {
	case STATUS_UNKNOWN:
		return "UNKNOWN"
	case STATUS_UP:
		return "UP"
	case STATUS_MAYBEDN:
		return "MaybeDOWN"
	case STATUS_DOWN:
		return "DOWN"
	case STATUS_SCEPTICAL:
		return "SCEPTICAL"
	case STATUS_DEAD:
		return "DEAD"
	}

	return "UNKOWN"
}
