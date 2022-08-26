package benchmark

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/ethdb/memorydb"
	"github.com/ethereum/go-ethereum/trie"
)

var (
	// emptyRoot is the known root hash of an empty trie.
	emptyRoot = common.HexToHash("56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421")
)

func init() {
	// log.Root().SetHandler(log.StreamHandler(os.Stderr, log.TerminalFormat(true)))
}

func MockWritesMPT(stdb state.Database, blocks int, writesPerContract int) (common.Hash, error) {
	root := common.Hash{}
	contracts := GenMockContracts()
	rootTrie, err := stdb.OpenTrie(common.Hash{})
	if err != nil {
		return common.Hash{}, err
	}

	for b := 0; b < blocks; b++ {
		for _, c := range contracts {
			trie, err := stdb.OpenStorageTrie(common.BytesToHash(c.Address.Bytes()), c.Root)
			if err != nil {
				return common.Hash{}, err
			}
			for k, v := range c.GenSlotUpdates(writesPerContract) {
				if err := trie.TryUpdate(k.Bytes(), v.Bytes()); err != nil {
					return common.Hash{}, err
				}
			}
			c.Root, _, err = trie.Commit(nil)
			if err != nil {
				return common.Hash{}, err
			}
			if err := rootTrie.TryUpdate(c.Address.Bytes(), c.Root.Bytes()); err != nil {
				return common.Hash{}, err
			}
		}
		root, _, err = rootTrie.Commit(func(_ [][]byte, _ []byte, leaf []byte, parent common.Hash) error {
			accountRoot := common.BytesToHash(leaf)
			if accountRoot != emptyRoot {
				stdb.TrieDB().Reference(accountRoot, parent)
			}
			return nil
		})

		// flush the db
		if err := stdb.TrieDB().Commit(root, true, nil); err != nil {
			panic(err)
		}
	}
	return root, nil
}

func BenchMPT() {
	kvdb := memorydb.New()
	db := rawdb.NewDatabase(kvdb)
	stdb := state.NewDatabaseWithConfig(db, &trie.Config{Preimages: false})
	_, err := MockWritesMPT(stdb, 100, 100)
	if err != nil {
		panic(err)
	}

	var size int
	iter := kvdb.NewIterator(nil, nil)
	for iter.Next() {
		size += len(iter.Key())
		size += len(iter.Value())
	}

	fmt.Println("mpt db size, nodes:", kvdb.Len(), "size:", size)
}
