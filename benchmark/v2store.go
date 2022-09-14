package benchmark

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/db/memdb"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2alpha1"
	"github.com/cosmos/cosmos-sdk/store/v2alpha1/multi"
)

var ModuleName = "evm"

func MockWritesV2Store(ms storetypes.CommitMultiStore, blocks int, writesPerContract int) error {
	contracts := GenMockContracts()
	for b := 0; b < blocks; b++ {
		kv := ms.GetKVStore(storetypes.NewKVStoreKey(ModuleName))
		for _, c := range contracts {
			for k, v := range c.GenSlotUpdates(writesPerContract) {
				kv.Set(k.Bytes(), v.Bytes())
			}
		}
		ms.Commit()
	}
	return nil
}

func BenchV2Store(blocks int, writesPerContract int) {
	db := memdb.NewDB()

	opts := multi.DefaultStoreConfig()
	opts.RegisterSubstore(ModuleName, storetypes.StoreTypePersistent)
	ms, err := multi.NewStore(db, opts)
	if err != nil {
		panic(err)
	}

	err = MockWritesV2Store(ms, blocks, writesPerContract)
	if err != nil {
		panic(err)
	}

	// sum the size of keys and values
	var size int
	iter, err := db.Reader().Iterator(nil, nil)
	if err != nil {
		panic(err)
	}
	for iter.Next() {
		size += len(iter.Key())
		size += len(iter.Value())
	}

	fmt.Println("v2store db size, nodes:", db.Stats()["database.size"], ", size:", size)
}
