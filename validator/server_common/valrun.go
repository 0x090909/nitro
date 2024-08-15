package server_common

import (
	"github.com/0x090909/nitro/util/containers"
	"github.com/0x090909/nitro/validator"
	"github.com/ethereum/go-ethereum/common"
)

type ValRun struct {
	containers.PromiseInterface[validator.GoGlobalState]
	root common.Hash
}

func (r *ValRun) WasmModuleRoot() common.Hash {
	return r.root
}

func NewValRun(promise containers.PromiseInterface[validator.GoGlobalState], root common.Hash) *ValRun {
	return &ValRun{
		PromiseInterface: promise,
		root:             root,
	}
}
