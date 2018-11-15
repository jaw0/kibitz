// Copyright (c) 2018
// Author: Jeff Weisberg <jaw @ tcp4me.com>
// Created: 2018-Nov-10 11:19 (EDT)
// Function: pick one at random

package kibitz

import (
	"math/rand"
)

type randPeer struct {
	count int
	p     *Peer
}
type randNet struct {
	count int
	p     *NetInfo
}

// ################################################################

func (rp *randPeer) maybe(p *Peer) {

	rp.count++

	if random_n(rp.count) == 0 {
		rp.p = p
	}
}

func (rp *randPeer) peer() *Peer {
	return rp.p
}

// ################################################################

func (rp *randNet) maybe(p *NetInfo) {

	rp.count++

	if random_n(rp.count) == 0 {
		rp.p = p
	}
}

func (rp *randNet) peer() *NetInfo {
	return rp.p
}

//################################################################

func random_n(n int) int {
	return int(rand.Int31n(int32(n)))
}
