// Copyright 2022-2024, Offchain Labs, Inc.
// For license information, see https://github.com/0x090909/nitro/blob/master/LICENSE

package programs

import (
	"errors"
	"math/big"

	"github.com/0x090909/nitro/arbos/addressSet"
	"github.com/0x090909/nitro/arbos/storage"
	"github.com/0x090909/nitro/arbos/util"
	"github.com/0x090909/nitro/arbutil"
	am "github.com/0x090909/nitro/util/arbmath"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
	gethParams "github.com/ethereum/go-ethereum/params"
)

type Programs struct {
	backingStorage *storage.Storage
	programs       *storage.Storage
	moduleHashes   *storage.Storage
	dataPricer     *DataPricer
	cacheManagers  *addressSet.AddressSet
}

type Program struct {
	version       uint16
	initCost      uint16
	cachedCost    uint16
	footprint     uint16
	asmEstimateKb uint24 // Predicted size of the asm
	activatedAt   uint24 // Hours since Arbitrum began
	ageSeconds    uint64 // Not stored in state
	cached        bool
}

type uint24 = am.Uint24

var paramsKey = []byte{0}
var programDataKey = []byte{1}
var moduleHashesKey = []byte{2}
var dataPricerKey = []byte{3}
var cacheManagersKey = []byte{4}

var ErrProgramActivation = errors.New("program activation failed")

var ProgramNotWasmError func() error
var ProgramNotActivatedError func() error
var ProgramNeedsUpgradeError func(version, stylusVersion uint16) error
var ProgramExpiredError func(age uint64) error
var ProgramUpToDateError func() error
var ProgramKeepaliveTooSoon func(age uint64) error

func Initialize(sto *storage.Storage) {
	initStylusParams(sto.OpenSubStorage(paramsKey))
	initDataPricer(sto.OpenSubStorage(dataPricerKey))
	_ = addressSet.Initialize(sto.OpenCachedSubStorage(cacheManagersKey))
}

func Open(sto *storage.Storage) *Programs {
	return &Programs{
		backingStorage: sto,
		programs:       sto.OpenSubStorage(programDataKey),
		moduleHashes:   sto.OpenSubStorage(moduleHashesKey),
		dataPricer:     openDataPricer(sto.OpenCachedSubStorage(dataPricerKey)),
		cacheManagers:  addressSet.OpenAddressSet(sto.OpenCachedSubStorage(cacheManagersKey)),
	}
}

func (p Programs) DataPricer() *DataPricer {
	return p.dataPricer
}

func (p Programs) CacheManagers() *addressSet.AddressSet {
	return p.cacheManagers
}

func (p Programs) CallProgram(
	scope *vm.ScopeContext,
	statedb vm.StateDB,
	arbosVersion uint64,
	interpreter *vm.EVMInterpreter,
	tracingInfo *util.TracingInfo,
	calldata []byte,
	reentrant bool,
	runmode core.MessageRunMode,
) ([]byte, error) {

	return nil, nil
}

func evmMemoryCost(size uint64) uint64 {
	// It would take 100GB to overflow this calculation, so no need to worry about that
	words := (size + 31) / 32
	linearCost := words * gethParams.MemoryGas
	squareCost := (words * words) / gethParams.QuadCoeffDiv
	return linearCost + squareCost
}

// Gets a program entry, which may be expired or not yet activated.
func (p Programs) getProgram(codeHash common.Hash, time uint64) (Program, error) {
	data, err := p.programs.Get(codeHash)
	program := Program{
		version:       am.BytesToUint16(data[:2]),
		initCost:      am.BytesToUint16(data[2:4]),
		cachedCost:    am.BytesToUint16(data[4:6]),
		footprint:     am.BytesToUint16(data[6:8]),
		activatedAt:   am.BytesToUint24(data[8:11]),
		asmEstimateKb: am.BytesToUint24(data[11:14]),
		cached:        am.BytesToBool(data[14:15]),
	}
	program.ageSeconds = hoursToAge(time, program.activatedAt)
	return program, err
}

// Gets a program entry. Errors if not active.
func (p Programs) getActiveProgram(codeHash common.Hash, time uint64, params *StylusParams) (Program, error) {
	program, err := p.getProgram(codeHash, time)
	if err != nil {
		return program, err
	}
	if program.version == 0 {
		return program, ProgramNotActivatedError()
	}

	// check that the program is up to date
	stylusVersion := params.Version
	if program.version != stylusVersion {
		return program, ProgramNeedsUpgradeError(program.version, stylusVersion)
	}

	// ensure the program hasn't expired
	if program.ageSeconds > am.DaysToSeconds(params.ExpiryDays) {
		return program, ProgramExpiredError(program.ageSeconds)
	}
	return program, nil
}

func (p Programs) setProgram(codehash common.Hash, program Program) error {
	data := common.Hash{}
	copy(data[0:], am.Uint16ToBytes(program.version))
	copy(data[2:], am.Uint16ToBytes(program.initCost))
	copy(data[4:], am.Uint16ToBytes(program.cachedCost))
	copy(data[6:], am.Uint16ToBytes(program.footprint))
	copy(data[8:], am.Uint24ToBytes(program.activatedAt))
	copy(data[11:], am.Uint24ToBytes(program.asmEstimateKb))
	copy(data[14:], am.BoolToBytes(program.cached))
	return p.programs.Set(codehash, data)
}

func (p Programs) programExists(codeHash common.Hash, time uint64, params *StylusParams) (uint16, bool, bool, error) {
	program, err := p.getProgram(codeHash, time)
	if err != nil {
		return 0, false, false, err
	}
	activatedAt := program.activatedAt
	expired := activatedAt == 0 || hoursToAge(time, activatedAt) > am.DaysToSeconds(params.ExpiryDays)
	return program.version, expired, program.cached, err
}

func (p Programs) ProgramKeepalive(codeHash common.Hash, time uint64, params *StylusParams) (*big.Int, error) {
	program, err := p.getActiveProgram(codeHash, time, params)
	if err != nil {
		return nil, err
	}
	if program.ageSeconds < am.DaysToSeconds(params.KeepaliveDays) {
		return nil, ProgramKeepaliveTooSoon(program.ageSeconds)
	}

	stylusVersion := params.Version
	if program.version != stylusVersion {
		return nil, ProgramNeedsUpgradeError(program.version, stylusVersion)
	}

	dataFee, err := p.dataPricer.UpdateModel(program.asmSize(), time)
	if err != nil {
		return nil, err
	}
	program.activatedAt = hoursSinceArbitrum(time)
	return dataFee, p.setProgram(codeHash, program)
}

// Gets whether a program is cached. Note that the program may be expired.
func (p Programs) ProgramCached(codeHash common.Hash) (bool, error) {
	data, err := p.programs.Get(codeHash)
	return am.BytesToBool(data[14:15]), err
}

// Sets whether a program is cached. Errors if trying to cache an expired program.
// `address` must be present if setting cache to true as of ArbOS 31,
// and if `address` is present it must have the specified codeHash.
func (p Programs) SetProgramCached(
	emitEvent func() error,
	db vm.StateDB,
	codeHash common.Hash,
	address common.Address,
	cache bool,
	time uint64,
	params *StylusParams,
	runMode core.MessageRunMode,
	debug bool,
) error {

	return nil
}

func (p Programs) CodehashVersion(codeHash common.Hash, time uint64, params *StylusParams) (uint16, error) {
	program, err := p.getActiveProgram(codeHash, time, params)
	if err != nil {
		return 0, err
	}
	return program.version, nil
}

// Gets the number of seconds left until expiration. Errors if it's already happened.
func (p Programs) ProgramTimeLeft(codeHash common.Hash, time uint64, params *StylusParams) (uint64, error) {
	program, err := p.getActiveProgram(codeHash, time, params)
	if err != nil {
		return 0, err
	}
	age := hoursToAge(time, program.activatedAt)
	expirySeconds := am.DaysToSeconds(params.ExpiryDays)
	if age > expirySeconds {
		return 0, ProgramExpiredError(age)
	}
	return am.SaturatingUSub(expirySeconds, age), nil
}

func (p Programs) ProgramInitGas(codeHash common.Hash, time uint64, params *StylusParams) (uint64, uint64, error) {
	program, err := p.getActiveProgram(codeHash, time, params)
	cachedGas := program.cachedGas(params)
	initGas := program.initGas(params)
	if params.Version > 1 {
		initGas += cachedGas
	}
	return initGas, cachedGas, err
}

func (p Programs) ProgramMemoryFootprint(codeHash common.Hash, time uint64, params *StylusParams) (uint16, error) {
	program, err := p.getActiveProgram(codeHash, time, params)
	return program.footprint, err
}

func (p Programs) ProgramAsmSize(codeHash common.Hash, time uint64, params *StylusParams) (uint32, error) {
	program, err := p.getActiveProgram(codeHash, time, params)
	if err != nil {
		return 0, err
	}
	return program.asmSize(), nil
}

func (p Program) asmSize() uint32 {
	return am.SaturatingUMul(p.asmEstimateKb.ToUint32(), 1024)
}

func (p Program) initGas(params *StylusParams) uint64 {
	base := uint64(params.MinInitGas) * MinInitGasUnits
	dyno := am.SaturatingUMul(uint64(p.initCost), uint64(params.InitCostScalar)*CostScalarPercent)
	return am.SaturatingUAdd(base, am.DivCeil(dyno, 100))
}

func (p Program) cachedGas(params *StylusParams) uint64 {
	base := uint64(params.MinCachedInitGas) * MinCachedGasUnits
	dyno := am.SaturatingUMul(uint64(p.cachedCost), uint64(params.CachedCostScalar)*CostScalarPercent)
	return am.SaturatingUAdd(base, am.DivCeil(dyno, 100))
}

type ProgParams struct {
	Version   uint16
	MaxDepth  uint32
	InkPrice  uint24
	DebugMode bool
}

func (p Programs) progParams(version uint16, debug bool, params *StylusParams) *ProgParams {
	return &ProgParams{
		Version:   version,
		MaxDepth:  params.MaxStackDepth,
		InkPrice:  params.InkPrice,
		DebugMode: debug,
	}
}

type EvmData struct {
	blockBasefee    common.Hash
	chainId         uint64
	blockCoinbase   common.Address
	blockGasLimit   uint64
	blockNumber     uint64
	blockTimestamp  uint64
	contractAddress common.Address
	moduleHash      common.Hash
	msgSender       common.Address
	msgValue        common.Hash
	txGasPrice      common.Hash
	txOrigin        common.Address
	reentrant       uint32
	cached          bool
	tracing         bool
}

type activationInfo struct {
	moduleHash    common.Hash
	initGas       uint16
	cachedInitGas uint16
	asmEstimate   uint32
	footprint     uint16
}

type userStatus uint8

const (
	userSuccess userStatus = iota
	userRevert
	userFailure
	userOutOfInk
	userOutOfStack
)

func (status userStatus) toResult(data []byte, debug bool) ([]byte, string, error) {
	msg := arbutil.ToStringOrHex(data)
	switch status {
	case userSuccess:
		return data, "", nil
	case userRevert:
		return data, msg, vm.ErrExecutionReverted
	case userFailure:
		return nil, msg, vm.ErrExecutionReverted
	case userOutOfInk:
		return nil, "", vm.ErrOutOfGas
	case userOutOfStack:
		return nil, "", vm.ErrDepth
	default:
		log.Error("program errored with unknown status", "status", status, "data", msg)
		return nil, msg, vm.ErrExecutionReverted
	}
}

// Hours since Arbitrum began, rounded down.
func hoursSinceArbitrum(time uint64) uint24 {
	return am.SaturatingUUCast[uint24]((am.SaturatingUSub(time, ArbitrumStartTime)) / 3600)
}

// Computes program age in seconds from the hours passed since Arbitrum began.
func hoursToAge(time uint64, hours uint24) uint64 {
	seconds := am.SaturatingUMul(uint64(hours), 3600)
	activatedAt := am.SaturatingUAdd(ArbitrumStartTime, seconds)
	return am.SaturatingUSub(time, activatedAt)
}
