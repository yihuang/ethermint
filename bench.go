package main

import "github.com/evmos/ethermint/benchmark"

func main() {
	blocks := 1000
	writesPerContract := 100
	benchmark.BenchIAVL(blocks, writesPerContract)
	benchmark.BenchMPT(blocks, writesPerContract)
}
