package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"time"

	"math/rand"

	"github.com/cmu440/bitcoin"
	"github.com/cmu440/lsp"
)

// This helper function receive an bitcoin.Message, marshal it, and write to the server
func MinerWriteMessage(client lsp.Client, message *bitcoin.Message) error {
	var buffer []byte
	buffer, err := json.Marshal(message)
	if err != nil {
		return err
	}
	err_write := client.Write(buffer)
	return err_write
}

// This helper function read from the server, unmarshal it and return the resulting message
func MinerReadMessage(client lsp.Client) (*bitcoin.Message, error) {
	payload, err := client.Read()
	if err != nil {
		return nil, err
	}
	var message bitcoin.Message
	err_unmarshal := json.Unmarshal(payload, &message)
	if err_unmarshal != nil {
		return nil, err_unmarshal
	}
	return &message, nil
}

// Attempt to connect miner as a client to the server.
func joinWithServer(hostport string) (lsp.Client, error) {
	// You will need this for randomized isn
	seed := rand.NewSource(time.Now().UnixNano())
	isn := rand.New(seed).Intn(int(math.Pow(2, 8)))

	client, err := lsp.NewClient(hostport, isn, lsp.NewParams())
	if err != nil {
		fmt.Println("Failed to connect to server:", err)
		return nil, errors.New("failed to Join")
	}
	joinMsg := bitcoin.NewJoin()
	MinerWriteMessage(client, joinMsg)
	return client, nil
}

// This function compute and return the minimum Hash with the associate nonce between lower and upper (inclusively)
func mining(miner lsp.Client, msg string, lower uint64, upper uint64) {
	resNonce := lower                   //The nonce with minimum hash
	resHash := bitcoin.Hash(msg, lower) // The minimum hash
	for i := lower; i < (upper + 1); i++ {
		currNonce, currHash := i, bitcoin.Hash(msg, i)
		if currHash < resHash {
			resNonce = currNonce
			resHash = currHash
		}
	}
	resMsg := bitcoin.NewResult(resHash, resNonce)
	MinerWriteMessage(miner, resMsg)
}

var LOGF *log.Logger

func main() {
	// You may need a logger for debug purpose
	const (
		name = "minerLog.txt"
		flag = os.O_RDWR | os.O_CREATE
		perm = os.FileMode(0666)
	)

	file, err := os.OpenFile(name, flag, perm)
	if err != nil {
		return
	}
	defer file.Close()

	const numArgs = 2
	if len(os.Args) != numArgs {
		fmt.Printf("Usage: ./%s <hostport>", os.Args[0])
		return
	}

	hostport := os.Args[1]
	miner, err := joinWithServer(hostport)
	if err != nil {
		fmt.Println("Failed to join with server:", err)
		return
	}

	defer miner.Close()

	for {
		//Miner keep reading from the work assignment from the server and return the result
		msg, err := MinerReadMessage(miner)
		if err != nil {
			fmt.Println("wrong message")
			miner.Close()
			break
		} else {
			//msg.Type would only be Request
			msg, lower, upper := msg.Data, msg.Lower, msg.Upper
			mining(miner, msg, lower, upper)
		}
	}
}
