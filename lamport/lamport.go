// Copyright (c) 2018
// Author: Jeff Weisberg <jaw @ tcp4me.com>
// Created: 2018-Aug-05 13:30 (EDT)
// Function: lamport clock that loosely tracks unix time

package lamport

import (
	"sync"
	"time"
)

type Time uint64

type Clock struct {
	lock sync.Mutex
	time Time
}

func New() *Clock {

	c := &Clock{
		time: wall(),
	}
	return c
}

func (c *Clock) Now() Time {
	return c.getInc(0)
}

func (c *Clock) Inc() Time {
	return c.getInc(1)
}

func (c *Clock) getInc(inc int) Time {

	now := wall()

	c.lock.Lock()
	defer c.lock.Unlock()

	if now > c.time {
		c.time = now
	}

	c.time += Time(inc)
	return c.time
}

func (c *Clock) Update(t Time) {

	c.lock.Lock()
	defer c.lock.Unlock()

	if t > c.time {
		c.time = t
	}
	c.time++
}

func (t Time) Uint64() uint64 {
	return uint64(t)
}
func ToTime(t uint64) Time {
	return Time(t)
}

func wall() Time {
	return Time(time.Now().UnixNano())
}
