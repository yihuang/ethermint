package statedb

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

// Keeper provide underlying storage of StateDB
type Keeper interface {
	// Read

	// returns nil if not exists
	// returns error if account is not EthAccount
	GetAccount(ctx sdk.Context, addr common.Address) (*Account, error)
	GetState(ctx sdk.Context, addr common.Address, key common.Hash) common.Hash
	GetCode(ctx sdk.Context, codeHash common.Hash) []byte
	GetCodeSize(ctx sdk.Context, codeHash common.Hash) int
	ForEachStorage(ctx sdk.Context, addr common.Address, cb func(key, value common.Hash) bool)

	// Write, only called by `StateDB.Commit()`
	SetAccount(ctx sdk.Context, addr common.Address, account Account) error
	SetState(ctx sdk.Context, addr common.Address, key, value common.Hash)
	SetCode(ctx sdk.Context, codeHash []byte, code []byte)
	PurgeAccount(ctx sdk.Context, addr common.Address) error
}
