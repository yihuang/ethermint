package benchmark

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/store/iavl"
	"github.com/cosmos/cosmos-sdk/store/prefix"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	evmtypes "github.com/evmos/ethermint/x/evm/types"
	dbm "github.com/tendermint/tm-db"
)

var (
	ModulePrefix = "s/k:evm/"
)

func MockWritesIAVL(kvstore storetypes.CommitKVStore, blocks int, writesPerContract int) error {
	contracts := GenMockContracts()
	for b := 0; b < blocks; b++ {
		for _, c := range contracts {
			store := prefix.NewStore(kvstore, evmtypes.AddressStoragePrefix(c.Address))
			for k, v := range c.GenSlotUpdates(writesPerContract) {
				store.Set(k.Bytes(), v.Bytes())
			}
		}
		kvstore.Commit()
	}
	return nil
}

func BenchIAVL() {
	db := dbm.NewMemDB()
	storeDB := dbm.NewPrefixDB(db, []byte(ModulePrefix))
	// storeKey := storetypes.NewKVStoreKey("evm")
	id := storetypes.CommitID{}
	store, err := iavl.LoadStore(storeDB, id, false, iavl.DefaultIAVLCacheSize)
	if err != nil {
		panic(err)
	}
	err = MockWritesIAVL(store, 100, 100)
	if err != nil {
		panic(err)
	}

	fmt.Println("iavl db size", db.Stats()["database.size"])
}
