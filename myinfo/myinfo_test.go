// Copyright (c) 2018
// Author: Jeff Weisberg <jaw @ tcp4me.com>
// Created: 2018-Nov-04 16:46 (EDT)
// Function:

package myinfo

import (
	"testing"
)

func tNew(x string) *Info {
	return &Info{clean: x}
}

func TestDatacenter(t *testing.T) {

	shouldBe(t, "sjc1", tNew("foo-r12.sjc1.domain.com").learnDatacenter())
	shouldBe(t, "domain", tNew("foo-r12.domain.com").learnDatacenter())
	shouldBe(t, "", tNew("foo.com").learnDatacenter())
}

func TestRack(t *testing.T) {

	shouldBe(t, "r12", tNew("foo-r12.sjc1.domain.com").learnRack())
	shouldBe(t, "", tNew("foo.sjc1.domain.com").learnRack())
}

func shouldBe(t *testing.T, a string, b string) {

	if a != b {
		t.Errorf("expected '%s', got '%s'\n", a, b)
	}
}
