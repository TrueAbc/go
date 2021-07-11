package datamodel

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"time"
)


type Block struct{
	Timestamp int64
	PrevBlockHash []byte
	Hash []byte
	Data []byte
}

func (b *Block)SetHash(){
	byteBuf := bytes.NewBuffer([]byte{})
	binary.Write(byteBuf, binary.BigEndian, b.Timestamp)


	cal := bytes.Join([][]byte{b.PrevBlockHash, b.Data, byteBuf.Bytes()}, []byte{})

	hash := sha256.Sum256(cal)

	b.Hash = hash[:]

}

func NewBlock(data string, prevHash []byte)*Block{
	b := &Block{Timestamp: time.Now().Unix(),
					PrevBlockHash: prevHash,
					Data: []byte(data),}

	b.SetHash()
	return b
}

func NewGenesisBlock()*Block{
	return NewBlock("genesis block", nil)
}