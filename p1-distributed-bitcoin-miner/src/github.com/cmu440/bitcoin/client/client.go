package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/cmu440/bitcoin"
	"github.com/cmu440/lsp"
)

var LOGF *log.Logger

//This helper function receive an bitcoin.Message, marshal it, and write to the server
func WriteMessage(client lsp.Client, message *bitcoin.Message) error {
	var buffer []byte
	buffer, err := json.Marshal(message)
	if err != nil {
		return err
	}
	err_write := client.Write(buffer)
	return err_write
}

//This helper function read from the server, unmarshal it and return the resulting message
func ReadMessage(client lsp.Client) (*bitcoin.Message, error) {
	payload, err := client.Read()
	if err != nil {
		return nil, err
	}
	var message bitcoin.Message
	err_unmarshal := json.Unmarshal(payload, &message)
	if err_unmarshal != nil {
		println("err_unmarshal")
		return nil, err_unmarshal
	}
	return &message, nil
}

func main() {
	const (
		name = "clientLog.txt"
		flag = os.O_RDWR | os.O_CREATE
		perm = os.FileMode(0666)
	)

	file, err := os.OpenFile(name, flag, perm)
	if err != nil {
		return
	}
	defer file.Close()

	const numArgs = 4
	if len(os.Args) != numArgs {
		fmt.Printf("Usage: ./%s <hostport> <message> <maxNonce>", os.Args[0])
		return
	}
	hostport := os.Args[1]
	message := os.Args[2]
	maxNonce, err := strconv.ParseUint(os.Args[3], 10, 64)
	if err != nil {
		fmt.Printf("%s is not a number.\n", os.Args[3])
		return
	}
	seed := rand.NewSource(time.Now().UnixNano())
	isn := rand.New(seed).Intn(int(math.Pow(2, 8)))

	client, err := lsp.NewClient(hostport, isn, lsp.NewParams())
	if err != nil {
		fmt.Println("Failed to connect to server:", err)
		return
	}

	defer client.Close()

	// send request to server
	request_msg := bitcoin.NewRequest(message, 0, maxNonce)
	err_write := WriteMessage(client, request_msg)
	if err_write != nil {
		printDisconnected()
		return
	}
	// read result from server
	result_msg, err_result := ReadMessage(client)
	if err_result != nil {
		printDisconnected()
		return
	}
	printResult(result_msg.Hash, result_msg.Nonce)
}

// printResult prints the final result to stdout.
func printResult(hash, nonce uint64) {
	fmt.Println("Result", hash, nonce)
}

// printDisconnected prints a disconnected message to stdout.
func printDisconnected() {
	fmt.Println("Disconnected")
}
