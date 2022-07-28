package benchmark

import (
	"math/big"
	"math/rand"

	"github.com/ethereum/go-ethereum/common"
)

type MockContract struct {
	Rnd      *rand.Rand
	Address  common.Address
	KeyRange int64
	Root     common.Hash
}

func (c MockContract) GenSlotUpdates(n int) map[common.Hash]common.Hash {
	slots := make(map[common.Hash]common.Hash, n)
	for i := 0; i < n; i++ {
		key := big.NewInt(c.Rnd.Int63n(c.KeyRange))
		value := big.NewInt(c.Rnd.Int63())
		slots[common.BigToHash(key)] = common.BigToHash(value)
	}
	return slots
}

func GenMockContracts() []MockContract {
	rnd := rand.New(rand.NewSource(100))
	return []MockContract{
		{Rnd: rnd, Address: common.BigToAddress(big.NewInt(1)), KeyRange: 100},
		{Rnd: rnd, Address: common.BigToAddress(big.NewInt(2)), KeyRange: 10000},
		{Rnd: rnd, Address: common.BigToAddress(big.NewInt(3)), KeyRange: 1000000},
	}
}
