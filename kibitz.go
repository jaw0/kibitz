// Copyright (c) 2018
// Author: Jeff Weisberg <jaw @ tcp4me.com>
// Created: 2018-Nov-05 12:58 (EST)
// Function: gossip protocol

package kibitz

import (
	"expvar"
	"time"

	"github.com/jaw0/acgo/diag"
)

const (
	TIMEOUT  = 15 * time.Second
	OLDTIMER = 9 * time.Minute // less than KEEPLOST
)

var dl = diag.Logger("kibitz")
var clientconns = expvar.NewInt("kibitz_client_reqs")
var clienterrs = expvar.NewInt("kibitz_client_fail")

func (pdb *DB) kibitzWithRandomPeer() {

	// randomly pick a peer
	peerAddr, natdom, peerId := pdb.kibitzPeer()
	if peerAddr == "" {
		dl.Debug("kibitz with peer - skipping - none")
		return
	}

	// don't talk to self. any of my addrs.
	if pdb.IsOwnAddr(peerAddr) {
		dl.Debug("kibitz with peer - skipping - not me %s %s", peerAddr, peerId)
		return
	}

	dl.Debug("kibitz with peer %s (%s)", peerAddr, peerId)

	_, err := pdb.iface.Send(peerAddr, TIMEOUT, pdb.MyInfo())

	if err != nil {
		dl.Debug(" => down err %v", err)
		pdb.PeerDn(peerId)

		clienterrs.Add(1)
		return
	}

	clientconns.Add(1)
	pdb.PeerUp(peerId)
	pdb.nmon.SetUp(natdom)
}

func (pdb *DB) getRandomPeer() *Peer {

	pdb.lock.RLock()
	defer pdb.lock.RUnlock()

	oldLimit := time.Now().Add(OLDTIMER)

	old := &randPeer{}
	local := &randPeer{}
	away := &randPeer{}
	check := &randPeer{}
	skept := &randPeer{}

	nall := 0

	for _, p := range pdb.kibitzers {
		pe := p.GetExport()
		nall++

		if pe.Status == STATUS_MAYBEDN {
			check.maybe(p)
		}

		if pe.LastTry.Before(oldLimit) {
			old.maybe(p)
		}

		if pe.Datacenter == pdb.dc {
			local.maybe(p)
		} else {
			away.maybe(p)
		}
	}

	for _, p := range pdb.skeptical {
		skept.maybe(p)
	}

	// first prefer anything sceptical
	usePeer := skept.p

	// then (maybe) anything pending
	maybeUse(usePeer, check.p, 5)

	// then (maybe) something about to expire
	maybeUse(usePeer, old.p, 5)

	// then (maybe) something far away
	k := 5
	if local.count < 5 {
		// not very many locally, use more far
		k = 2
	}
	if usePeer == nil && away.p != nil && random_n(k) == 0 {
		usePeer = away.p
	}

	// otherwise prefer local
	if usePeer == nil {
		usePeer = local.p
	}

	// sometimes, use seed. so we can recover from a partition
	if random_n(2*nall+2) == 0 {
		usePeer = nil
	}

	return usePeer
}

func (pdb *DB) kibitzPeer() (string, string, string) {

	p := pdb.getRandomPeer()

	if p != nil {
		return pdb.useAddr(p)
	}

	nseed := len(pdb.seed)

	if nseed != 0 {
		return pdb.seed[random_n(nseed)], "[seed]", "[seed]"
	}

	return "", "", ""
}

func maybeUse(curr *Peer, nxt *Peer, n int) *Peer {

	if nxt == nil {
		return curr
	}
	if curr == nil || random_n(n) == 0 {
		return nxt
	}
	return curr
}

func (pdb *DB) useAddr(p *Peer) (string, string, string) {

	down := &randNet{}
	public := &randNet{}
	private := &randNet{}

	for _, na := range p.getAddrs() {
		dom := na.GetNatdom()
		isup, known := pdb.nmon.IsUp(dom)
		if !known {
			// remote private network
			continue
		}
		if isup {
			if dom == "" {
				public.maybe(na)
			} else {
				private.maybe(na)
			}
		} else {
			down.maybe(na)
		}
	}

	// preference (but sometimes mix it up): private (cheaper), public, down (to test if it is still down)

	prefer := private.p
	if prefer == nil || random_n(20) == 0 {
		prefer = public.p
	}
	if prefer == nil || random_n(20) == 0 {
		prefer = down.p
	}

	if prefer == nil {
		return "", "", ""
	}

	return prefer.GetAddr(), prefer.GetNatdom(), p.id
}
