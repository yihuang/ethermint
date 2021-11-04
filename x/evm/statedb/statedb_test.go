package statedb_test

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"github.com/tharsis/ethermint/x/evm/statedb"
)

var _ statedb.Keeper = &MockKeeper{}

type MockKeeper struct {
	errAddress common.Address

	accounts map[common.Address]statedb.Account
	states   map[common.Address]statedb.Storage
	codes    map[common.Hash][]byte
}

func NewMockKeeper() *MockKeeper {
	return &MockKeeper{
		errAddress: common.BigToAddress(big.NewInt(1)),

		accounts: make(map[common.Address]statedb.Account),
		states:   make(map[common.Address]statedb.Storage),
		codes:    make(map[common.Hash][]byte),
	}
}

func (k MockKeeper) GetAccount(ctx sdk.Context, addr common.Address) (*statedb.Account, error) {
	if addr == k.errAddress {
		return nil, errors.New("mock db error")
	}
	acct, ok := k.accounts[addr]
	if !ok {
		return nil, nil
	}
	return &acct, nil
}

func (k MockKeeper) GetState(ctx sdk.Context, addr common.Address, key common.Hash) common.Hash {
	return k.states[addr][key]
}

func (k MockKeeper) GetCode(ctx sdk.Context, codeHash common.Hash) []byte {
	return k.codes[codeHash]
}

func (k MockKeeper) GetCodeSize(ctx sdk.Context, codeHash common.Hash) int {
	code := k.GetCode(ctx, codeHash)
	return len(code)
}

func (k MockKeeper) ForEachStorage(ctx sdk.Context, addr common.Address, cb func(key, value common.Hash) bool) {
	for k, v := range k.states[addr] {
		if cb(k, v) {
			return
		}
	}
}

func (k MockKeeper) SetAccount(ctx sdk.Context, addr common.Address, account statedb.Account) error {
	fmt.Println("set account", account)
	k.accounts[addr] = account
	return nil
}

func (k MockKeeper) SetState(ctx sdk.Context, addr common.Address, key, value common.Hash) {
	k.states[addr][key] = value
}

func (k MockKeeper) SetCode(ctx sdk.Context, codeHash []byte, code []byte) {
	k.codes[common.BytesToHash(codeHash)] = code
}

func (k MockKeeper) PurgeAccount(ctx sdk.Context, addr common.Address) error {
	old := k.accounts[addr]
	delete(k.accounts, addr)
	delete(k.states, addr)
	if len(old.CodeHash) > 0 {
		delete(k.codes, common.BytesToHash(old.CodeHash))
	}
	return nil
}

func TestAccounts(t *testing.T) {
	addrErr := common.BigToAddress(big.NewInt(1))
	addr2 := common.BigToAddress(big.NewInt(2))
	testTxConfig := statedb.NewTxConfig(
		common.BigToHash(big.NewInt(10)), // tx hash
		common.BigToHash(big.NewInt(11)), // block hash
		1,                                // txIndex
		1,                                // logSize
	)

	testCases := []struct {
		msg  string
		test func(*testing.T, *statedb.StateDB)
	}{
		{
			"success,empty account",
			func(t *testing.T, db *statedb.StateDB) {
				require.Equal(t, true, db.Empty(addr2))
				require.Equal(t, big.NewInt(0), db.GetBalance(addr2))
				require.Equal(t, []byte(nil), db.GetCode(addr2))
				require.Equal(t, uint64(0), db.GetNonce(addr2))
			},
		},
		{
			"success,GetBalance",
			func(t *testing.T, db *statedb.StateDB) {
				db.AddBalance(addr2, big.NewInt(1))
				require.Equal(t, big.NewInt(1), db.GetBalance(addr2))
			},
		},
		{
			"fail,GetBalance dbErr",
			func(t *testing.T, db *statedb.StateDB) {
				require.Equal(t, big.NewInt(0), db.GetBalance(addrErr))
				require.Error(t, db.Commit())
			},
		},
		{
			"success,change balance",
			func(t *testing.T, db *statedb.StateDB) {
				db.AddBalance(addr2, big.NewInt(2))
				require.Equal(t, big.NewInt(2), db.GetBalance(addr2))
				db.SubBalance(addr2, big.NewInt(1))
				require.Equal(t, big.NewInt(1), db.GetBalance(addr2))

				require.NoError(t, db.Commit())

				// create a clean StateDB, check the balance is committed
				db = statedb.New(db.Context(), db.Keeper(), testTxConfig)
				require.Equal(t, big.NewInt(1), db.GetBalance(addr2))
			},
		},
		{
			"success,SetState",
			func(t *testing.T, db *statedb.StateDB) {
				key := common.BigToHash(big.NewInt(1))
				value := common.BigToHash(big.NewInt(1))

				require.Equal(t, common.Hash{}, db.GetState(addr2, key))
				db.SetState(addr2, key, value)
				require.Equal(t, value, db.GetState(addr2, key))
				require.Equal(t, common.Hash{}, db.GetCommittedState(addr2, key))
			},
		},
		{
			"success,SetCode",
			func(t *testing.T, db *statedb.StateDB) {
				code := []byte("hello world")
				codeHash := crypto.Keccak256Hash(code)
				db.SetCode(addr2, code)
				require.Equal(t, code, db.GetCode(addr2))
				require.Equal(t, codeHash, db.GetCodeHash(addr2))

				require.NoError(t, db.Commit())

				// create a clean StateDB, check the code is committed
				db = statedb.New(db.Context(), db.Keeper(), testTxConfig)
				require.Equal(t, code, db.GetCode(addr2))
				require.Equal(t, codeHash, db.GetCodeHash(addr2))
			},
		},
		{
			"success,CreateAccount",
			func(t *testing.T, db *statedb.StateDB) {
				// test balance carry over when overwritten
				amount := big.NewInt(1)
				code := []byte("hello world")
				key := common.BigToHash(big.NewInt(1))
				value := common.BigToHash(big.NewInt(1))

				db.AddBalance(addr2, amount)
				db.SetCode(addr2, code)
				db.SetState(addr2, key, value)

				rev := db.Snapshot()

				db.CreateAccount(addr2)
				require.Equal(t, amount, db.GetBalance(addr2))
				require.Equal(t, []byte(nil), db.GetCode(addr2))
				require.Equal(t, common.Hash{}, db.GetState(addr2, key))

				db.RevertToSnapshot(rev)
				require.Equal(t, amount, db.GetBalance(addr2))
				require.Equal(t, code, db.GetCode(addr2))
				require.Equal(t, value, db.GetState(addr2, key))

				db.CreateAccount(addr2)
				require.NoError(t, db.Commit())
				db = statedb.New(db.Context(), db.Keeper(), testTxConfig)
				require.Equal(t, amount, db.GetBalance(addr2))
				require.Equal(t, []byte(nil), db.GetCode(addr2))
				require.Equal(t, common.Hash{}, db.GetState(addr2, key))
			},
		},
		{
			"success,nested snapshot revert",
			func(t *testing.T, db *statedb.StateDB) {
				key := common.BigToHash(big.NewInt(1))
				value1 := common.BigToHash(big.NewInt(1))
				value2 := common.BigToHash(big.NewInt(2))

				rev1 := db.Snapshot()
				db.SetState(addr2, key, value1)

				rev2 := db.Snapshot()
				db.SetState(addr2, key, value2)
				require.Equal(t, value2, db.GetState(addr2, key))

				db.RevertToSnapshot(rev2)
				require.Equal(t, value1, db.GetState(addr2, key))

				db.RevertToSnapshot(rev1)
				require.Equal(t, common.Hash{}, db.GetState(addr2, key))
			},
		},
		{
			"success,nonce",
			func(t *testing.T, db *statedb.StateDB) {
				require.Equal(t, uint64(0), db.GetNonce(addr2))
				db.SetNonce(addr2, 1)
				require.Equal(t, uint64(1), db.GetNonce(addr2))

				require.NoError(t, db.Commit())

				db = statedb.New(db.Context(), db.Keeper(), testTxConfig)
				require.Equal(t, uint64(1), db.GetNonce(addr2))
			},
		},
		{
			"success,logs",
			func(t *testing.T, db *statedb.StateDB) {
				data := []byte("hello world")
				db.AddLog(&ethtypes.Log{
					Address:     addr2,
					Topics:      []common.Hash{},
					Data:        data,
					BlockNumber: 1,
				})
				require.Equal(t, 1, len(db.Logs()))
				expecedLog := &ethtypes.Log{
					Address:     addr2,
					Topics:      []common.Hash{},
					Data:        data,
					BlockNumber: 1,
					BlockHash:   common.BigToHash(big.NewInt(10)),
					TxHash:      common.BigToHash(big.NewInt(11)),
					TxIndex:     1,
					Index:       1,
				}
				require.Equal(t, expecedLog, db.Logs()[0])

				rev := db.Snapshot()

				db.AddLog(&ethtypes.Log{
					Address:     addr2,
					Topics:      []common.Hash{},
					Data:        data,
					BlockNumber: 1,
				})
				require.Equal(t, 2, len(db.Logs()))
				require.Equal(t, uint(2), db.Logs()[1].Index)

				db.RevertToSnapshot(rev)
				require.Equal(t, 1, len(db.Logs()))
			},
		},
		{
			"success,refund",
			func(t *testing.T, db *statedb.StateDB) {
				db.AddRefund(uint64(10))
				require.Equal(t, uint64(10), db.GetRefund())

				rev := db.Snapshot()

				db.SubRefund(uint64(5))
				require.Equal(t, uint64(5), db.GetRefund())

				db.RevertToSnapshot(rev)
				require.Equal(t, uint64(10), db.GetRefund())
			},
		},
		{
			"success,empty",
			func(t *testing.T, db *statedb.StateDB) {
				require.False(t, db.Exist(addr2))
				require.True(t, db.Empty(addr2))

				db.AddBalance(addr2, big.NewInt(1))
				require.True(t, db.Exist(addr2))
				require.False(t, db.Empty(addr2))

				db.SubBalance(addr2, big.NewInt(1))
				require.True(t, db.Exist(addr2))
				require.True(t, db.Empty(addr2))
			},
		},
		{
			"success,suicide commit",
			func(t *testing.T, db *statedb.StateDB) {
				code := []byte("hello world")
				db.SetCode(addr2, code)
				db.AddBalance(addr2, big.NewInt(1))

				require.True(t, db.Exist(addr2))
				require.False(t, db.Empty(addr2))

				db.Suicide(addr2)
				require.True(t, db.HasSuicided(addr2))
				require.True(t, db.Exist(addr2))
				require.Equal(t, new(big.Int), db.GetBalance(addr2))

				db.Commit()
				db = statedb.New(db.Context(), db.Keeper(), testTxConfig)
				require.True(t, db.Empty(addr2))
			},
		},
		{
			"success,suicide revert",
			func(t *testing.T, db *statedb.StateDB) {
				code := []byte("hello world")
				db.SetCode(addr2, code)
				db.AddBalance(addr2, big.NewInt(1))

				rev := db.Snapshot()

				db.Suicide(addr2)
				require.True(t, db.HasSuicided(addr2))

				db.RevertToSnapshot(rev)

				require.False(t, db.HasSuicided(addr2))
				require.Equal(t, code, db.GetCode(addr2))
				require.Equal(t, big.NewInt(1), db.GetBalance(addr2))
			},
		},
		// TODO access list, ForEachStorage
	}
	for _, tc := range testCases {
		t.Run(tc.msg, func(t *testing.T) {
			db := statedb.New(
				sdk.Context{},
				NewMockKeeper(),
				testTxConfig,
			)
			tc.test(t, db)
		})
	}
}
