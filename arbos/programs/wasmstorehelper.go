// Copyright 2022-2024, Offchain Labs, Inc.
// For license information, see https://github.com/nitro/blob/master/LICENSE

//go:build !wasm
// +build !wasm

package programs

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
)

// SaveActiveProgramToWasmStore is used to save active stylus programs to wasm store during rebuilding
func (p Programs) SaveActiveProgramToWasmStore(statedb *state.StateDB, codeHash common.Hash, code []byte, time uint64, debugMode bool, rebuildingStartBlockTime uint64) error {
	return nil
}
