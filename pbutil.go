// Copyright (c) 2018
// Author: Jeff Weisberg <jaw @ tcp4me.com>
// Created: 2018-Nov-05 10:16 (EST)
// Function:

package kibitz

import (
//"github.com/golang/protobuf/proto"
)

func (p *PeerInfo) SetStatusCode(st PeerStatus) {
	p.StatusCode = int32(st)
}
