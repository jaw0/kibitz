// Copyright (c) 2022
// Author: Jeff Weisberg <tcp4me.com!jaw>
// Created: 2022-Apr-11 13:58 (EDT)
// Function: example app using json post api

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/jaw0/acgo/diag"
	"github.com/jaw0/enginz"
	"github.com/jaw0/kibitz"
)

var dl = diag.Logger("testapp")

type pinfo struct{}

var pdb *kibitz.DB

func main() {
	var port int
	var seed string
	flag.StringVar(&seed, "s", "", "seed addr")
	flag.IntVar(&port, "p", 8901, "port")

	flag.Parse()

	var seeds []string
	if seed != "" {
		seeds = []string{seed}
	}

	pdb = kibitz.New(&kibitz.Conf{
		System:      "testy",
		Environment: "dev",
		Datacenter:  "us-east",
		Rack:        "r1",
		Port:        port,
		Seed:        seeds,
		Iface:       pinfo{},
	})

	// run a simple web server
	httpz := &enginz.Server{
		Report:   diag.Logger("http"),
		ServerID: "testapp",
		Service:  []enginz.Service{{Addr: fmt.Sprintf(":%d", port)}},
		Handler: enginz.Routes{
			"/kibitz": recvKibitz, // api endpoint
		},
	}

	pdb.Start()
	httpz.Serve()

}

// our API sends/recvs these:
type HBRequest struct {
	Myself *HB
}
type HBResponse struct {
	Status int
	Infos  []*HB
}
type HB struct {
	Info       *kibitz.PeerInfo
	SampleData string
}

// structs must implement the interface
var _ kibitz.PeerImport = &HB{}

func (h *HB) GetPeerInfo() *kibitz.PeerInfo {
	return h.Info
}
func (h *HB) SetPeerInfo(info *kibitz.PeerInfo) {
	h.Info = info
}

// talk to remote server
func (pinfo) Send(addr string, timeout time.Duration, myInfo *kibitz.PeerInfo) ([]kibitz.PeerImport, error) {
	dl.Debug("send: %s %#v", addr, myInfo)

	// build request
	req := HBRequest{
		Myself: &HB{
			SampleData: "hello!",
			Info:       myInfo,
		},
	}

	// make json post api call
	url := fmt.Sprintf("http://%s/kibitz", addr)
	res := HBResponse{}
	err := jsonPost(url, timeout, req, &res)

	if err != nil {
		return nil, err
	}

	if res.Status != 200 {
		return nil, fmt.Errorf("hb/status %d", res.Status)
	}

	// build results
	respi := make([]kibitz.PeerImport, len(res.Infos))
	for i, hb := range res.Infos {
		respi[i] = kibitz.PeerImport(hb)
	}

	return respi, err
}

func (pinfo) Notify(id string, isup bool, isMySys bool) {
	state := "down"
	if isup {
		state = "up"
	}
	dl.Verbose("notify: %s => %s", id, state)
}

// handle incoming json post request
func recvKibitz(w http.ResponseWriter, req *http.Request) {

	dl.Debug("request from %s", req.RemoteAddr)

	if req.Method != "POST" {
		w.WriteHeader(405)
		return
	}

	// process incoming request
	hbreq := HBRequest{}
	err := json.NewDecoder(req.Body).Decode(&hbreq)

	if err != nil {
		dl.Verbose("cannot decode request: %v", err)
		w.WriteHeader(500)
		return
	}

	if hbreq.Myself != nil {
		// add this peer to the db
		pdb.UpdateSceptical(hbreq.Myself)
	}

	// build reply
	res := HBResponse{Status: 200}

	pdb.ForAllData(func(pd interface{}) {
		w := pd.(*HB)
		res.Infos = append(res.Infos, w)
	})

	// add myself to reply
	self := &HB{
		SampleData: "hello!",
		Info:       pdb.MyInfo(),
	}
	res.Infos = append(res.Infos, self)

	js, _ := json.Marshal(res)
	w.Write(js)
}

func jsonPost(url string, timeout time.Duration, req interface{}, res interface{}) error {

	js, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest("POST", url, bytes.NewBuffer(js))
	httpReq.Header.Set("Content-Type", "application/json; charset=UTF-8")

	client := &http.Client{Timeout: timeout}

	resp, err := client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("server replied %s", resp.Status)
	}

	err = json.NewDecoder(resp.Body).Decode(res)
	if err != nil {
		return err
	}

	return nil
}
