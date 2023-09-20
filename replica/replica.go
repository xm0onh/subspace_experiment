package replica

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/xm0onh/subspace_experiment/blockchain"
	"github.com/xm0onh/subspace_experiment/config"
	"github.com/xm0onh/subspace_experiment/election"
	"github.com/xm0onh/subspace_experiment/identity"
	"github.com/xm0onh/subspace_experiment/log"
	"github.com/xm0onh/subspace_experiment/mempool"
	"github.com/xm0onh/subspace_experiment/operator"
)

type Replica struct {
	operator.Operator
	election.Election
	mem             *mempool.Producer
	committedBlocks chan *blockchain.Block
	eventChan       chan interface{}
	roundNo         int
	Inter
}

func NewReplica(id identity.NodeID) *Replica {
	r := new(Replica)
	r.Operator = operator.NewOperator(id)
	r.Election = election.NewRotation(config.GetConfig().N())
	r.mem = mempool.NewProducer()
	r.roundNo = 0
	r.eventChan = make(chan interface{})
	r.committedBlocks = make(chan *blockchain.Block, 1000)
	r.Register(blockchain.Block{}, r.HandleBlock)
	gob.Register(blockchain.Block{})

	r.Inter = blockchain.NewSubpace(r.Operator, r.Election, r.committedBlocks)
	return r
}

func ExtractJSON(data string) (string, error) {
	startIndex := strings.Index(data, "{\"Proposer\"")
	if startIndex == -1 {
		return "", fmt.Errorf("JSON content not found")
	}

	return data[startIndex:], nil
}

func StringToBlock(data string) (*blockchain.Block, error) {
	extractedJSON, err := ExtractJSON(data)
	if err != nil {
		return nil, err
	}

	var block blockchain.Block
	err = json.Unmarshal([]byte(extractedJSON), &block)
	if err != nil {
		return nil, err
	}
	return &block, nil
}

func (r *Replica) HandleBlock(block blockchain.Block) {

	// if !r.IsLeader(r.ID(), r.roundNo) {
	// 	r.roundNo++
	// }
	log.Debugf("[%v] received a block from %v, view is %v, id: %x, prevID: %x", r.ID(), block.Proposer, block.View, block.ID, block.PrevID)
	_ = r.Inter.ProcessBlock(r.ID(), &block)

}

func (r *Replica) NewComingBlock() {
	for {

		if r.Operator.RecvT() != "" {

			// StringTOBlock

			block, err := StringToBlock(r.Operator.RecvT())
			// time.Sleep(1 * time.Second)
			if err == nil {
				r.HandleBlock(*block)
				r.Operator.SetT()
				r.roundNo++
				// fmt.Println("round, ", r.roundNo)
			}
			// msg := blockchain.Block.FromString(o.test)
			// v := reflect.ValueOf(msg)
			// name := v.Type().String()
			// f, exists := o.handles[name]
			// if !exists {
			// 	log.Fatalf("no registered handle function for message type %v", name)
			// }

			// f.Call([]reflect.Value{v})
		}
	}

}

func (r *Replica) proposeBlock(view int) {

	if r.IsLeader(r.ID(), r.roundNo) {
		block := blockchain.NewBlock(r.ID(), view, r.roundNo, r.roundNo-1, r.mem.GetTransactions())
		// fmt.Println("next leader is:" + r.FindLeaderFor(r.roundNo+1))
		_ = r.Inter.ProcessBlock(r.ID(), block)
		r.Broadcast(block.ToString())
		r.roundNo++
	}

}

func (r *Replica) Start() {
	go r.Run()
	go r.NewComingBlock()
	for i := 0; i < 100; i++ {
		fmt.Println("I am node", r.ID(), "and I received a block in round", r.roundNo-1)
		r.proposeBlock(r.roundNo)
		time.Sleep(1000 * time.Millisecond)
	}

	// if r.IsLeader(r.ID(), r.roundNo) && r.roundNo == 0 {
	// 	r.proposeBlock(r.roundNo)
	// 	fmt.Println("Hello")
	// }
	// for {

	// 	event := <-r.eventChan
	// 	if block, ok := event.(blockchain.Block); ok {
	// 		_ = r.Inter.ProcessBlock(r.ID(), &block)
	// 		// r.Inter.ProcessBlock(r.ID(), &block)
	// 		fmt.Println("I am node", r.ID(), "and I received a block in round", r.roundNo-1)
	// 		fmt.Println("Leader for", r.roundNo, "is:-->", r.FindLeaderFor(r.roundNo))
	// 		fmt.Println("----My view is", r.roundNo-1)
	// 		fmt.Println()
	// 		r.roundNo++

	// 		startTime := time.Now()
	// 		for {
	// 			// Check if 200 milliseconds have passed
	// 			if time.Since(startTime) >= 500*time.Millisecond {
	// 				r.roundNo++
	// 				r.proposeBlock(r.roundNo)
	// 				break
	// 			}
	// 			r.proposeBlock(r.roundNo)
	// 		}

	// 	}
	// }

}
