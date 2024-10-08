// Copyright 2021-2022, Offchain Labs, Inc.
// For license information, see https://github.com/nitro/blob/master/LICENSE

package staker

import (
	"github.com/0x090909/nitro/validator"
	"github.com/ethereum/go-ethereum/common"
)

type legacyLastBlockValidatedDbInfo struct {
	BlockNumber   uint64
	BlockHash     common.Hash
	AfterPosition GlobalStatePosition
}

type GlobalStateValidatedInfo struct {
	GlobalState validator.GoGlobalState
	WasmRoots   []common.Hash
}

var (
	lastGlobalStateValidatedInfoKey = []byte("_lastGlobalStateValidatedInfo") // contains a rlp encoded lastBlockValidatedDbInfo
	legacyLastBlockValidatedInfoKey = []byte("_lastBlockValidatedInfo")       // LEGACY - contains a rlp encoded lastBlockValidatedDbInfo
)
