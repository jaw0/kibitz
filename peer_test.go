// Copyright (c) 2018
// Author: Jeff Weisberg <jaw @ tcp4me.com>
// Created: 2018-Nov-05 08:12 (EST)
// Function:

package kibitz

import (
	"fmt"
	"testing"
)

func TestPeer(t *testing.T) {

	pdb := New(&Conf{
		System:      "mrtesty",
		Environment: "test",
		Hostname:    "u12-r14.phlccs1.example.com",
		Port:        1234,
	})

	if pdb.Rack() != "r14" || pdb.Datacenter() != "phlccs1" {
		t.Fatalf("pdb: %#v\n", pdb)
	}
	if pdb.Id() != "mrtesty/test/1234@u12-r14.phlccs1.example.com" {
		t.Fatalf("pdb: %#v\n", pdb)
	}

	if false {
		fmt.Printf("ok %#v\n", pdb)
	}
}
