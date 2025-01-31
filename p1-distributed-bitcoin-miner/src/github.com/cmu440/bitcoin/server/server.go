package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/cmu440/bitcoin"
	"github.com/cmu440/lsp"
)

type server struct {
	lspServer     lsp.Server
	free_miner    []int              // current free miner
	work_miner    map[int]int        // working miner: miner_id --> client_id
	task_map      map[int]*task_data // assigned request: client_id --> info of task distribution
	request_queue []*request_data    // unassigned request: request data
}

// store information for each request
type request_data struct {
	client_id int    //client's id
	data      string // the message
	lower     uint64 // the lowest nonce in a request
	upper     uint64 // the highest nonce in a request
}

type miner_data struct {
	lower uint64 //lower: the lowest nonce a miner has to mine in a task
	upper uint64 //upper: the highest nonce a miner has to mine in a task
}

// store information of each assigned task
type task_data struct {
	data       string              //the message
	miner_num  int                 // the number of miner assigned
	miner_info map[int]*miner_data // miner_id --> working interval
	curr_value uint64              // store result
	curr_nonce uint64              // nonce associate with curr_value
}

// Initialize the server
func startServer(port int) (*server, error) {
	params := lsp.NewParams()
	server_lsp, err := lsp.NewServer(port, params)
	if err != nil {
		return nil, err
	}
	new_server := &server{
		server_lsp,
		make([]int, 0),
		make(map[int]int),
		make(map[int]*task_data),
		make([]*request_data, 0),
	}
	return new_server, nil
}

// The server read, unmarshal, and return the next message
func ServerReadMessage(srv *server) (int, *bitcoin.Message, error) {
	id, payload, err := srv.lspServer.Read()
	if err != nil {
		return id, nil, err
	}
	var message bitcoin.Message
	err_unmarshal := json.Unmarshal(payload, &message)
	if err_unmarshal != nil {
		return id, nil, err_unmarshal
	}
	return id, &message, nil
}

// The server marshal and send the message
func ServerWriteMessage(srv *server, id int, message *bitcoin.Message) error {
	var buffer []byte
	buffer, err := json.Marshal(message)
	if err != nil {
		return err
	}
	err_write := srv.lspServer.Write(id, buffer)
	return err_write
}

var LOGF *log.Logger

func main() {
	const (
		name = "serverLog.txt"
		flag = os.O_RDWR | os.O_CREATE
		perm = os.FileMode(0666)
	)

	file, err := os.OpenFile(name, flag, perm)
	if err != nil {
		return
	}
	defer file.Close()

	LOGF = log.New(file, "", log.Lshortfile|log.Lmicroseconds)

	const numArgs = 2
	if len(os.Args) != numArgs {
		fmt.Printf("Usage: ./%s <port>", os.Args[0])
		return
	}

	port, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Println("Port must be a number:", err)
		return
	}

	srv, err := startServer(port)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Println("Server listening on port", port)

	defer srv.lspServer.Close()

	for {
		// read the message
		id, message, err := ServerReadMessage(srv)

		if err != nil { // fail handling
			unassigned_disconnect := false
			for i, re_data := range srv.request_queue {
				if re_data.client_id == id { // if this client's request has not been assigned to a miner, then delete the request from request_queue
					srv.request_queue = append(srv.request_queue[:i], srv.request_queue[i+1:]...)
					unassigned_disconnect = true
					break
				}
			}
			_, exists := srv.task_map[id]
			if exists { // if this client's request has already been assigned, then remove it from the srv.task_map
				delete(srv.task_map, id) // remove from task_map
			}
			//If it is the error from a miner
			if !(exists || unassigned_disconnect) {
				// check whether it is a free miner id
				exists_free_miner := false
				for i, free_id := range srv.free_miner {
					if free_id == id {
						// remove from free miner list
						srv.free_miner = append(srv.free_miner[:i], srv.free_miner[i+1:]...)
						exists_free_miner = true
						break
					}
				}
				//If the faulty miner it is already working, then reassign the work to other miner and delete the faulty miner
				if !exists_free_miner {
					cid, exists_work_miner := srv.work_miner[id]
					if exists_work_miner { // it is a working miner id
						// get info for task it works on
						task_info := srv.task_map[cid]
						task_interval := task_info.miner_info[id]
						low, upper := task_interval.lower, task_interval.upper
						task_data := task_info.data
						// delete this miner
						delete(srv.task_map[cid].miner_info, id)
						srv.task_map[cid].miner_num -= 1
						delete(srv.work_miner, id)
						if len(srv.free_miner) > 0 { // there is free miner, reassign the work
							miner_id := srv.free_miner[0]
							srv.free_miner = srv.free_miner[1:]
							request_msg := bitcoin.NewRequest(task_data, low, upper)
							ServerWriteMessage(srv, miner_id, request_msg)
							srv.work_miner[miner_id] = cid // record what task the miner is doing
							srv.task_map[cid].miner_info[miner_id] = &miner_data{low, upper}
							srv.task_map[cid].miner_num += 1
						} else { // no free miner right now
							request := []*request_data{{cid, task_data, low, upper}}
							srv.request_queue = append(request, srv.request_queue...)
						}
					}
				}
			}
		} else {
			// Load Balancing Strategy: We use a round-robin strategy to maintain fairness and optimize performance.
			// Specifically, client requests are placed in a queue. When a miner becomes available, it is assigned the first request in the queue,
			// processing up to a maximum size of 10,000 units. If the request is not fully processed, the remaining portion is stored,
			// and the request is moved to the end of the queue for future processing.
			switch message.Type {
			case bitcoin.Join:
				// a new miner joins
				// need to update for scheduler
				if len(srv.request_queue) == 0 { // no pending request
					srv.free_miner = append(srv.free_miner, id) // add to free miner
				} else { // there are pending requests
					request := srv.request_queue[0]
					cid, data, low, upper := request.client_id, request.data, request.lower, request.upper
					size := upper - low + 1
					if size <= 10000 { // small request, send all at once
						request_msg := bitcoin.NewRequest(data, low, upper)
						ServerWriteMessage(srv, id, request_msg)
						srv.work_miner[id] = cid      // record what task the miner is doing
						_, exist := srv.task_map[cid] // check whether it is part of an assigned task
						if exist {
							// if so, update task_map
							srv.task_map[cid].miner_info[id] = &miner_data{low, upper}
							srv.task_map[cid].miner_num += 1
						} else { // store into task_map
							info := make(map[int]*miner_data)
							info[id] = &miner_data{low, upper}
							srv.task_map[cid] = &task_data{data, 1, info, 0, 0}
						}
						// remove from request queue after sending
						srv.request_queue = srv.request_queue[1:]
					} else { // large request, send the first 10000
						request_msg := bitcoin.NewRequest(data, low, low+10000)
						ServerWriteMessage(srv, id, request_msg)
						srv.work_miner[id] = cid      // record what task the miner is doing
						_, exist := srv.task_map[cid] // check whether it is part of a larger task
						if exist {
							// if so, update task_map
							srv.task_map[cid].miner_info[id] = &miner_data{low, low + 10000}
							srv.task_map[cid].miner_num += 1
						} else { // store into task_map
							info := make(map[int]*miner_data)
							info[id] = &miner_data{low, low + 10000}
							srv.task_map[cid] = &task_data{data, 1, info, 0, 0}
						}
						new_request := &request_data{cid, data, low + 10000, upper}
						srv.request_queue = append(srv.request_queue[1:], new_request)
					}
				}
			case bitcoin.Request:
				data, lower, upper := message.Data, message.Lower, message.Upper
				size := upper - lower + 1
				free_miner_num := len(srv.free_miner)
				if free_miner_num == 0 { // there is no free miner, store to request queue
					request := &request_data{id, data, lower, upper}
					srv.request_queue = append(srv.request_queue, request)
				} else { // there are some free miner
					if size <= 10000 { // small request, send all at once
						miner_id := srv.free_miner[0]
						srv.free_miner = srv.free_miner[1:]
						request_msg := bitcoin.NewRequest(data, lower, upper)
						ServerWriteMessage(srv, miner_id, request_msg)
						srv.work_miner[miner_id] = id // record what task the miner is doing
						info := make(map[int]*miner_data)
						info[miner_id] = &miner_data{lower, upper}
						srv.task_map[id] = &task_data{data, 1, info, 0, 0}
					} else { // large request, send as much as we can
						lower_bound := lower
						for len(srv.free_miner) > 0 && uint64(lower_bound) < upper {
							miner_id := srv.free_miner[0]
							srv.free_miner = srv.free_miner[1:]
							miner_low := lower_bound
							miner_upper := lower_bound + 10000
							if miner_upper > upper {
								miner_upper = upper
							}
							request_msg := bitcoin.NewRequest(data, miner_low, miner_upper)
							ServerWriteMessage(srv, miner_id, request_msg)
							srv.work_miner[miner_id] = id // record what task the miner is doing
							task_info, exists := srv.task_map[id]
							if !exists {
								info := make(map[int]*miner_data)
								info[miner_id] = &miner_data{miner_low, miner_upper}
								srv.task_map[id] = &task_data{data, 1, info, 0, 0}
							} else {
								task_info.miner_info[miner_id] = &miner_data{miner_low, miner_upper}
								task_info.miner_num += 1
							}
							lower_bound = miner_upper
						}
						if len(srv.free_miner) == 0 && lower_bound < upper { // save remaining
							request := &request_data{id, data, lower_bound, upper}
							srv.request_queue = append(srv.request_queue, request)
						}
					}

				}

			case bitcoin.Result:
				// get task_info we stored
				value, nonce := message.Hash, message.Nonce
				cid := srv.work_miner[id] // find the corresponding task id
				task_info, exist := srv.task_map[cid]
				if exist {
					delete(task_info.miner_info, id) // remove from unreceived miner
					task_info.miner_num -= 1         // decrement unreceived miner number
				}
				// deal with this free miner
				if len(srv.request_queue) == 0 {
					srv.free_miner = append(srv.free_miner, id) // add back to free miner
					delete(srv.work_miner, id)                  // remove from working miner
				} else { // assign work
					request := srv.request_queue[0]
					cid_request, data, low, upper := request.client_id, request.data, request.lower, request.upper
					size := upper - low + 1
					if size <= 10000 { // small request, send all at once
						request_msg := bitcoin.NewRequest(data, low, upper)
						ServerWriteMessage(srv, id, request_msg)
						srv.work_miner[id] = cid_request      // record what task the miner is doing
						_, exist := srv.task_map[cid_request] // check whether it is part of a larger task
						if exist {
							// if so, update task_map
							srv.task_map[cid_request].miner_info[id] = &miner_data{low, upper}
							srv.task_map[cid_request].miner_num += 1
						} else { // store into task_map
							info := make(map[int]*miner_data)
							info[id] = &miner_data{low, upper}
							srv.task_map[cid_request] = &task_data{data, 1, info, 0, 0}
						}
						// remove from request queue after sending
						srv.request_queue = srv.request_queue[1:]
					} else { // large request, send the first 10000
						request_msg := bitcoin.NewRequest(data, low, low+10000)
						ServerWriteMessage(srv, id, request_msg)
						srv.work_miner[id] = cid_request      // record what task the miner is doing
						_, exist := srv.task_map[cid_request] // check whether it is part of a larger task
						if exist {
							// if so, update task_map
							srv.task_map[cid_request].miner_info[id] = &miner_data{low, low + 10000}
							srv.task_map[cid_request].miner_num += 1
						} else { // store into task_map
							info := make(map[int]*miner_data)
							info[id] = &miner_data{low, low + 10000}
							srv.task_map[cid_request] = &task_data{data, 1, info, 0, 0}
						}
						// update the queue after sending
						// 1. move to the back
						// 2. remove the sent 10k
						new_request := &request_data{cid_request, data, low + 10000, upper}
						srv.request_queue = append(srv.request_queue[1:], new_request)
					}
				}
				// update hash value for this task
				if exist {
					if task_info.curr_value == 0 || value < task_info.curr_value {
						task_info.curr_value = value
						task_info.curr_nonce = nonce
					}
					if task_info.miner_num == 0 { // received from all miners assigned this task
						result_msg := bitcoin.NewResult(task_info.curr_value, task_info.curr_nonce)
						ServerWriteMessage(srv, cid, result_msg)
						delete(srv.task_map, cid) // remove from task_map
					}
				}
			}
		}
	}
}
