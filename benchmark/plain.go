package benchmark

import (
	"fmt"

	dbm "github.com/tendermint/tm-db"
)

func MockWritesPlain(db dbm.DB, blocks int, writesPerContract int) error {
	contracts := GenMockContracts()
	for b := 0; b < blocks; b++ {
		for _, c := range contracts {
			for k, v := range c.GenSlotUpdates(writesPerContract) {
				db.Set(k.Bytes(), v.Bytes())
			}
		}
	}
	return nil
}

func BenchPlain(blocks int, writesPerContract int) {
	db := dbm.NewMemDB()
	pdb := dbm.NewPrefixDB(db, []byte(ModulePrefix))

	err := MockWritesPlain(pdb, blocks, writesPerContract)
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

	fmt.Println("plain db size, nodes:", db.Stats()["database.size"], ", size:", size)
}
