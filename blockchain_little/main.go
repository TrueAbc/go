package main

import (
	"fmt"

	"github.com/go/blockchain_little/datamodel"
)

func main() {
	bc := datamodel.NewBlockChain()
	bc.AddBlock("send 1 btc to abc")
	bc.AddBlock("send 2 more btc to abc")

	for _, block := range bc.Blocks {
		fmt.Printf("Prev. hash: %x\n", block.PrevBlockHash)
		fmt.Printf("Data: %s\n", block.Data)
		fmt.Printf("Hash: %x\n", block.Hash)
		fmt.Println()
	}
}
