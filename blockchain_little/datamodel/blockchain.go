package datamodel

type Blockchain struct{
	Blocks []*Block
}

func (bc *Blockchain)AddBlock(data string){
	prevHash := bc.Blocks[len(bc.Blocks)-1]
	newBlock := NewBlock(data, prevHash.Hash)

	bc.Blocks = append(bc.Blocks, newBlock)
}

func NewBlockChain()*Blockchain{
	genesis := NewGenesisBlock()
	bc := &Blockchain{make([]*Block, 0)}

	bc.Blocks = append(bc.Blocks, genesis)
	return bc
}

