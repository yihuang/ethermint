package benchmark

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/store/iavl"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	"github.com/tendermint/tendermint/libs/log"
	dbm "github.com/tendermint/tm-db"
)

var (
	ModulePrefix = "s/k:evm/"
)

func MockWritesIAVL(store storetypes.CommitKVStore, blocks int, writesPerContract int) error {
	contracts := GenMockContracts()
	for b := 0; b < blocks; b++ {
		for _, c := range contracts {
			for k, v := range c.GenSlotUpdates(writesPerContract) {
				store.Set(k.Bytes(), v.Bytes())
			}
		}
		store.Commit()
	}
	return nil
}

func BenchIAVL(blocks int, writesPerContract int) {
	db := dbm.NewMemDB()
	storeDB := dbm.NewPrefixDB(db, []byte(ModulePrefix))
	// storeKey := storetypes.NewKVStoreKey("evm")
	id := storetypes.CommitID{}
	store, err := iavl.LoadStore(storeDB, log.NewNopLogger(), storetypes.NewKVStoreKey("evm"), id, false, iavl.DefaultIAVLCacheSize)
	if err != nil {
		panic(err)
	}
	err = MockWritesIAVL(store, blocks, writesPerContract)
	if err != nil {
		panic(err)
	}

	// sum the size of keys and values
	var size int
	iter, _ := db.Iterator(nil, nil)
	for {
		if !iter.Valid() {
			break
		}
		size += len(iter.Key())
		size += len(iter.Value())
		iter.Next()
	}

	fmt.Println("iavl db size, nodes:", db.Stats()["database.size"], ", size:", size)
}
