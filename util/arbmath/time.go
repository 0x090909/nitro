// Copyright 2024, Offchain Labs, Inc.
// For license information, see https://github.com/0x090909/nitro/blob/master/LICENSE

package arbmath

func DaysToSeconds[T Unsigned](days T) uint64 {
	return uint64(days) * 24 * 60 * 60
}
