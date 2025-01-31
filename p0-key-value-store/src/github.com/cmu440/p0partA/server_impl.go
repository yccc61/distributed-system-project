// Implementation of a KeyValueServer. Students should write their code in this file.

package p0partA

import (
	"bufio"
	"bytes"
	"net"
	"strconv"

	"github.com/cmu440/p0partA/kvstore"
)

type keyValueServer struct {
	store            kvstore.KVStore
	listener         net.Listener
	connections      chan net.Conn
	keyValue_request chan *kv_request
	clients          []*client
	activeClient     chan int

	dropped_client      int
	dropped_client_chan chan int

	close_signal chan bool
}

type client struct {
	connection net.Conn
	readChan   chan []byte
	writeChan  chan []byte
}

type kv_request struct {
	currentClient *client
	request_type  []byte
	key           string
	old_value     []byte
	new_value     []byte
}

// New creates and returns (but does not start) a new KeyValueServer.
// Your keyValueServer should use `store kvstore.KVStore` internally.
func New(store kvstore.KVStore) KeyValueServer {
	return &keyValueServer{store: store, listener: nil,
		connections: make(chan net.Conn), keyValue_request: make(chan *kv_request),
		clients: make([]*client, 0, 10), activeClient: make(chan int),
		dropped_client: 0, dropped_client_chan: make(chan int), close_signal: make(chan bool)}
}

func accept(kvs *keyValueServer) {
	//initialize new channels with other client
	for {
		select {
		case <-kvs.close_signal:
			return
		default:
			conn, err := kvs.listener.Accept()
			if err != nil {
				return
			} else {
				kvs.connections <- conn
			}
		}

	}

}

func mainRoutine(kvs *keyValueServer) {
	for {
		select {
		case <-kvs.close_signal:
			for _, c := range kvs.clients {
				err := c.connection.Close()
				if err != nil {
					return
				}
			}
			return
		case connection := <-kvs.connections:
			read_chan := make(chan []byte)
			write_chan := make(chan []byte, 500)
			newClient := &client{connection, read_chan, write_chan}
			kvs.clients = append(kvs.clients, newClient)
			go readRoutine(kvs, newClient)
			go writeRoutine(newClient)
		case kv_request := <-kvs.keyValue_request:
			if bytes.Equal(kv_request.request_type, []byte("Put")) {
				kvs.store.Put(kv_request.key, kv_request.new_value)
			} else if bytes.Equal(kv_request.request_type, []byte("Get")) {
				result := kvs.store.Get(kv_request.key)
				for _, v := range result {
					message := append([]byte(kv_request.key), ':')
					message = append(message, v...)
					message = append(message, '\n')
					if len(kv_request.currentClient.writeChan) < 500 {
						//only send message when the lenth of the buffer is less than 500
						//otherwise the message is dropped.
						kv_request.currentClient.writeChan <- message
					}
				}
			} else if bytes.Equal(kv_request.request_type, []byte("Delete")) {
				kvs.store.Delete(kv_request.key)
			} else if bytes.Equal(kv_request.request_type, []byte("Update")) {
				kvs.store.Update(kv_request.key, kv_request.old_value, kv_request.new_value)
			} else if bytes.Equal(kv_request.request_type, []byte("Terminate")) {
				kvs.dropped_client += 1
				index_of_client := -1
				for i, c := range kvs.clients {
					if c == kv_request.currentClient {
						index_of_client = i
						break
					}
				}
				kvs.clients[index_of_client].connection.Close()
				kvs.clients = append(kvs.clients[:index_of_client], kvs.clients[index_of_client+1:]...)

			}
		case <-kvs.activeClient:
			kvs.activeClient <- len(kvs.clients)

		case <-kvs.dropped_client_chan:
			kvs.dropped_client_chan <- kvs.dropped_client
		}
	}
}

func (kvs *keyValueServer) Start(port int) error {
	listener, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		return err
	}
	kvs.listener = listener

	go accept(kvs)
	go mainRoutine(kvs)

	return nil
}

func readRoutine(kvs *keyValueServer, currClient *client) {
	currReader := bufio.NewReader(currClient.connection)
	for {
		request, err := currReader.ReadBytes('\n')
		//Trim the newline when reading the new request.
		request = bytes.Trim(request, string('\n'))
		if err != nil {
			//Disconnect the client
			kvs.keyValue_request <- &kv_request{
				currentClient: currClient,
				request_type:  []byte("Terminate"),
			}
			return
		}

		parsed_request := bytes.Split(request, []byte(":"))
		request_type := parsed_request[0]
		if bytes.Equal(request_type, []byte("Put")) {
			kvs.keyValue_request <- &kv_request{
				currentClient: currClient,
				request_type:  []byte("Put"),
				key:           string(parsed_request[1]),
				new_value:     parsed_request[2],
			}
		} else if bytes.Equal(request_type, []byte("Get")) {
			kvs.keyValue_request <- &kv_request{
				currentClient: currClient,
				request_type:  []byte("Get"),
				key:           string(parsed_request[1]),
			}
		} else if bytes.Equal(request_type, []byte("Delete")) {
			kvs.keyValue_request <- &kv_request{
				currentClient: currClient,
				request_type:  []byte("Delete"),
				key:           string(parsed_request[1]),
			}
		} else if bytes.Equal(request_type, []byte("Update")) {
			kvs.keyValue_request <- &kv_request{
				currentClient: currClient,
				request_type:  []byte("Update"),
				key:           string(parsed_request[1]),
				old_value:     parsed_request[2],
				new_value:     parsed_request[3],
			}
		}
	}

}

func writeRoutine(currClient *client) {
	for {
		select {
		//write routine writes the message received to the client
		case message := <-currClient.writeChan:
			currClient.connection.Write(message)
		}
	}

}

func (kvs *keyValueServer) Close() {
	kvs.close_signal <- true
}

func (kvs *keyValueServer) CountActive() int {
	//When count active is called, it send 1 to the channel to indicate request for the task
	kvs.activeClient <- 1
	result := <-kvs.activeClient
	return result
}

func (kvs *keyValueServer) CountDropped() int {
	//when count dropped is called, it send 1 to the channel to indicate request for the task.
	kvs.dropped_client_chan <- 1
	result := <-kvs.dropped_client_chan
	return result
}
