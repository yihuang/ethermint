package statedb

import (
	"bytes"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

var emptyCodeHash = crypto.Keccak256(nil)

// Account is the Ethereum consensus representation of accounts.
// These objects are stored in the main account trie.
type Account struct {
	Nonce    uint64
	Balance  *big.Int
	CodeHash []byte
}

func NewEmptyAccount() *Account {
	return &Account{
		Balance:  new(big.Int),
		CodeHash: emptyCodeHash,
	}
}

type Storage map[common.Hash]common.Hash

// SortedKeys sort the keys for deterministic iteration
func (s Storage) SortedKeys() []common.Hash {
	keys := make([]common.Hash, len(s))
	i := 0
	for k := range s {
		keys[i] = k
		i++
	}
	sort.Slice(keys, func(i, j int) bool {
		return bytes.Compare(keys[i].Bytes(), keys[j].Bytes()) < 0
	})
	return keys
}

// stateObject is the state of an acount
type stateObject struct {
	db *StateDB

	account Account
	code    []byte

	// state storage
	originStorage Storage
	dirtyStorage  Storage

	// DB error.
	// State objects are used by the consensus core and VM which are
	// unable to deal with database-level errors. Any error that occurs
	// during a database read is memoized here and will eventually be returned
	// by StateDB.Commit.

	address common.Address

	// flags
	dirtyCode bool
	suicided  bool
}

// newObject creates a state object.
func newObject(db *StateDB, address common.Address, account Account) *stateObject {
	if account.Balance == nil {
		account.Balance = new(big.Int)
	}
	if account.CodeHash == nil {
		account.CodeHash = emptyCodeHash
	}
	return &stateObject{
		db:            db,
		address:       address,
		account:       account,
		originStorage: make(Storage),
		dirtyStorage:  make(Storage),
	}
}

// empty returns whether the account is considered empty.
func (s *stateObject) empty() bool {
	return s.account.Nonce == 0 && s.account.Balance.Sign() == 0 && bytes.Equal(s.account.CodeHash, emptyCodeHash)
}

func (s *stateObject) markSuicided() {
	s.suicided = true
}

// AddBalance adds amount to s's balance.
// It is used to add funds to the destination account of a transfer.
func (s *stateObject) AddBalance(amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	s.SetBalance(new(big.Int).Add(s.Balance(), amount))
}

// SubBalance removes amount from s's balance.
// It is used to remove funds from the origin account of a transfer.
func (s *stateObject) SubBalance(amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	s.SetBalance(new(big.Int).Sub(s.Balance(), amount))
}

func (s *stateObject) SetBalance(amount *big.Int) {
	s.db.journal.append(balanceChange{
		account: &s.address,
		prev:    new(big.Int).Set(s.account.Balance),
	})
	s.setBalance(amount)
}

func (s *stateObject) setBalance(amount *big.Int) {
	s.account.Balance = amount
}

// Return the gas back to the origin. Used by the Virtual machine or Closures
func (s *stateObject) ReturnGas(gas *big.Int) {}

//
// Attribute accessors
//

// Returns the address of the contract/account
func (s *stateObject) Address() common.Address {
	return s.address
}

// Code returns the contract code associated with this object, if any.
func (s *stateObject) Code() []byte {
	if s.code != nil {
		return s.code
	}
	if bytes.Equal(s.CodeHash(), emptyCodeHash) {
		return nil
	}
	code := s.db.keeper.GetCode(s.db.ctx, common.BytesToHash(s.CodeHash()))
	s.code = code
	return code
}

// CodeSize returns the size of the contract code associated with this object,
// or zero if none. This method is an almost mirror of Code, but uses a cache
// inside the database to avoid loading codes seen recently.
func (s *stateObject) CodeSize() int {
	if s.code != nil {
		return len(s.code)
	}
	if bytes.Equal(s.CodeHash(), emptyCodeHash) {
		return 0
	}
	return s.db.keeper.GetCodeSize(s.db.ctx, common.BytesToHash(s.CodeHash()))
}

func (s *stateObject) SetCode(codeHash common.Hash, code []byte) {
	prevcode := s.Code()
	s.db.journal.append(codeChange{
		account:  &s.address,
		prevhash: s.CodeHash(),
		prevcode: prevcode,
	})
	s.setCode(codeHash, code)
}

func (s *stateObject) setCode(codeHash common.Hash, code []byte) {
	s.code = code
	s.account.CodeHash = codeHash[:]
	s.dirtyCode = true
}

func (s *stateObject) SetNonce(nonce uint64) {
	s.db.journal.append(nonceChange{
		account: &s.address,
		prev:    s.account.Nonce,
	})
	s.setNonce(nonce)
}

func (s *stateObject) setNonce(nonce uint64) {
	s.account.Nonce = nonce
}

func (s *stateObject) CodeHash() []byte {
	return s.account.CodeHash
}

func (s *stateObject) Balance() *big.Int {
	return s.account.Balance
}

func (s *stateObject) Nonce() uint64 {
	return s.account.Nonce
}

// GetCommittedState query the committed state
func (s *stateObject) GetCommittedState(key common.Hash) common.Hash {
	if value, cached := s.originStorage[key]; cached {
		return value
	}
	// If no live objects are available, load it from keeper
	value := s.db.keeper.GetState(s.db.ctx, s.Address(), key)
	s.originStorage[key] = value
	return value
}

// GetState query the current state (including dirty state)
func (s *stateObject) GetState(key common.Hash) common.Hash {
	if value, dirty := s.dirtyStorage[key]; dirty {
		return value
	}
	return s.GetCommittedState(key)
}

func (s *stateObject) SetState(key common.Hash, value common.Hash) {
	// If the new value is the same as old, don't set
	prev := s.GetState(key)
	if prev == value {
		return
	}
	// New value is different, update and journal the change
	s.db.journal.append(storageChange{
		account:  &s.address,
		key:      key,
		prevalue: prev,
	})
	s.setState(key, value)
}

func (s *stateObject) setState(key, value common.Hash) {
	s.dirtyStorage[key] = value
}
