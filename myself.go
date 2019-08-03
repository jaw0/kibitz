// Copyright (c) 2018
// Author: Jeff Weisberg <jaw @ tcp4me.com>
// Created: 2018-Nov-07 13:05 (EDT)
// Function:

package kibitz

import (
	"github.com/jaw0/acgo/diag"
	"github.com/jaw0/kibitz/myinfo"
)

var viaDot = "."

var dlme = diag.Logger("kibitz_myself")

func (pdb *DB) learn(c *Conf) {

	myself := myinfo.GetInfo(pdb.host)

	if pdb.id == "" {
		pdb.id = myself.ServerId(pdb.sys, pdb.env, pdb.port)
	}

	if pdb.host == "" {
		pdb.host = myself.Hostname
	}
	if pdb.dc == "" {
		pdb.dc = myself.Datacenter
	}
	if pdb.rack == "" {
		pdb.rack = myself.Rack
	}

	dlme.Debug("server id: %s", pdb.id)
	dlme.Debug("dc: %s, r %s", pdb.dc, pdb.rack)

	// netinfo
	pdb.learnNetwork()
}

func (pdb *DB) learnNetwork() {

	ninfo := myinfo.Network(pdb.dc, pdb.port)

	for _, ni := range ninfo {
		pdb.myaddrs[ni.Addr] = ni.Dom
		pdb.mydoms[ni.Dom] = true
		pdb.nmon.Add(ni.Dom)
		pdb.bestaddr = ni.Addr

		dlme.Debug("intf %s [%s]", ni.Addr, ni.Dom)

		a := ni.Addr
		d := ni.Dom

		pdb.netinfo = append(pdb.netinfo, &NetInfo{
			Addr:   a,
			Natdom: d,
		})

	}
}

func (pdb *DB) MyInfo() *PeerInfo {

	now := pdb.clock.Inc().Uint64()

	r := &PeerInfo{
		Subsystem:   pdb.sys,
		Environment: pdb.env,
		ServerId:    pdb.id,
		Hostname:    pdb.host,
		NetInfo:     pdb.netinfo,
		TimeCreated: now,
		TimeChecked: now,
		TimeLastUp:  now,
		TimeUpSince: pdb.bootTime,
		TimeConf:    pdb.bootTime,
		Via:         viaDot,
	}

	r.SetStatusCode(STATUS_UP)

	if pdb.dc != "" {
		r.Datacenter = pdb.dc
	}
	if pdb.rack != "" {
		r.Rack = pdb.rack
	}

	return r
}

func (pdb *DB) IsOwnAddr(addr string) bool {
	_, ok := pdb.myaddrs[addr]
	return ok
}
