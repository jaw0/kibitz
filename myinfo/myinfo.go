// Copyright (c) 2018
// Author: Jeff Weisberg <jaw @ tcp4me.com>
// Created: 2018-Nov-04 16:18 (EST)
// Function: who am I?

package myinfo

import (
	"fmt"
	"net"
	"os"
	"strings"
)

type NetInfo struct {
	Dom  string
	Addr string
}

type Info struct {
	clean      string
	Hostname   string
	Datacenter string
	Rack       string
	Id         string
	Net        []NetInfo
	myaddrs    map[string]bool
}

func GetInfo(host string) *Info {

	info := &Info{
		myaddrs: make(map[string]bool),
	}

	info.Hostname = host
	if info.Hostname == "" {
		info.Hostname, _ = os.Hostname()
	}
	info.clean = normalizeName(info.Hostname)

	info.learnDatacenter()
	info.learnRack()

	return info
}

func (i *Info) ServerId(sys string, env string, port int) string {

	id := sys
	if env != "prod" && env != "" {
		id = id + "/" + env + "/" + fmt.Sprintf("%d", port)
	}
	id = id + "@" + i.clean

	return id
}

// ################################################################

func normalizeName(host string) string {

	if strings.HasSuffix(host, ".local") {
		return host[0 : len(host)-len(".local")]
	}
	return host
}

// parse from hostname "name.datacenter.domain"
func (i *Info) learnDatacenter() string {

	parts := strings.Split(i.clean, ".")

	if len(parts) > 2 {
		i.Datacenter = parts[1]
	}
	return i.Datacenter
}

// parse from hostname "name-r#..."
func (i *Info) learnRack() string {

	name := i.clean
	s0 := strings.Index(name, "-r")
	if s0 == -1 {
		return ""
	}
	s0++ // <->r => -<r>

	s1 := strings.Index(name[s0:], ".")

	if s1 == -1 {
		i.Rack = name[s0:]
	} else {
		i.Rack = name[s0 : s0+s1]
	}

	return i.Rack
}

func Network(dom string, port int) []NetInfo {

	var ni []NetInfo
	intfs, _ := net.Interfaces()

	for _, i := range intfs {
		if i.Flags&net.FlagLoopback != 0 {
			continue
		}
		if i.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, _ := i.Addrs()

		for _, a := range addrs {
			ip, blk, _ := net.ParseCIDR(a.String())
			if !ip.IsGlobalUnicast() {
				continue
			}

			var ipport string

			if len(ip.To4()) == 4 {
				ipport = fmt.Sprintf("%s:%d", ip.String(), port)
			} else {
				ipport = fmt.Sprintf("[%s]:%d", ip.String(), port)
			}

			natdom := ""
			if isPrivateIP(ip) {
				// use datacenter name or netblock
				// QQQ - use concatenation dc+blk - will people reuse the same net in multiple dcs?
				natdom = dom

				if natdom == "" {
					natdom = blk.String()
				}
			}

			ni = append(ni, NetInfo{Dom: natdom, Addr: ipport})
		}
	}

	return ni
}

func (i *Info) IsOwnAddr(addr string) bool {
	return i.myaddrs[addr]
}

var pvtRange = [...]string{
	"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", // RFC 1918
	"fc00::/7", // RFC 4193
}

func isPrivateIP(ip net.IP) bool {

	if ip == nil {
		return false
	}

	for _, n := range pvtRange {
		_, block, _ := net.ParseCIDR(n)
		if block.Contains(ip) {
			return true
		}
	}
	return false

}
