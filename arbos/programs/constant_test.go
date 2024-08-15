// Copyright 2024, Offchain Labs, Inc.
// For license information, see https://github.com/0x090909/nitro/blob/master/LICENSE

package programs

import "testing"

func TestConstants(t *testing.T) {
	err := testConstants()
	if err != nil {
		t.Fatal(err)
	}
}
