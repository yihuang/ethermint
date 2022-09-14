package main

import "github.com/evmos/ethermint/benchmark"

func main() {
	blocks := 1000
	writesPerContract := 100
	benchmark.BenchPlain(blocks, writesPerContract)
	benchmark.BenchIAVL(blocks, writesPerContract)
	benchmark.BenchMPT(blocks, writesPerContract)
	benchmark.BenchV2Store(blocks, writesPerContract)
}
