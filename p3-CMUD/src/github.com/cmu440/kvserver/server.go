// Package kvserver implements the backend server for a
// geographically distributed, highly available, NoSQL key-value store.
package kvserver

import (
	"encoding/json"
	"fmt"
	"net"
	"net/rpc"

	"github.com/cmu440/actor"
)

// A single server in the key-value store, running some number of
// query actors - nominally one per CPU core. Each query actor
// provides a key/value storage service on its own port.
//
// Different query actors (both within this server and across connected
// servers) periodically sync updates (Puts) following an eventually
// consistent, last-writer-wins strategy.
type Server struct {
	actorSystem *actor.ActorSystem
	actorList   []*actor.ActorRef
}

// OPTIONAL: Error handler for ActorSystem.OnError.
//
// Print the error or call debug.PrintStack() in this function.
// When starting an ActorSystem, call ActorSystem.OnError(errorHandler).
// This can help debug server-side errors more easily.
func errorHandler(err error) {
}

// Starts a server running queryActorCount query actors.
//
// The server's actor system listens for remote messages (from other actor
// systems) on startPort. The server listens for RPCs from kvclient.Clients
// on ports [startPort + 1, startPort + 2, ..., startPort + queryActorCount].
// Each of these "query RPC servers" answers queries by asking a specific
// query actor.
//
// remoteDescs contains a "description" string for each existing server in the
// key-value store. Specifically, each slice entry is the desc returned by
// an existing server's own NewServer call. The description strings are opaque
// to callers, but typically an implementation uses JSON-encoded data containing,
// e.g., actor.ActorRef's that remote servers' actors should contact.
//
// Before returning, NewServer starts the ActorSystem, all query actors, and
// all query RPC servers. If there is an error starting anything, that error is
// returned instead.
func NewServer(startPort int, queryActorCount int, remoteDescs []string) (server *Server, desc string, err error) {
	// TODO (3A, 3B): implement this!

	// Tips:
	// - The "HTTP service" example in the net/rpc docs does not support
	// multiple RPC servers in the same process. Instead, use the following
	// template to start RPC servers (adapted from
	// https://groups.google.com/g/Golang-Nuts/c/JTn3LV_bd5M/m/cMO_DLyHPeUJ ):
	//
	//  rpcServer := rpc.NewServer()
	//  err := rpcServer.RegisterName("QueryReceiver", [*queryReceiver instance])
	//  ln, err := net.Listen("tcp", ...)
	//  go func() {
	//    for {
	//      conn, err := ln.Accept()
	//      if err != nil {
	//        return
	//      }
	//      go rpcServer.ServeConn(conn)
	//    }
	//  }()
	//
	// - To start query actors, call your ActorSystem's
	// StartActor(newQueryActor), where newQueryActor is defined in ./query_actor.go.
	// Do this queryActorCount times. (For the checkpoint tests,
	// queryActorCount will always be 1.)
	// - remoteDescs and desc: see doc comment above.
	// For the checkpoint, it is okay to ignore remoteDescs and return "" for desc.

	system, err := actor.NewActorSystem(startPort)
	if err != nil {
		return nil, "", err
	}
	actorList := make([]*actor.ActorRef, 0)
	for i := 0; i < queryActorCount; i++ {
		rpcServer := rpc.NewServer()
		ref := system.StartActor(newQueryActor)
		// queryReceiver := &queryReceiver{ref: ref, system: system}
		queryReceiver := NewQueryReceiver(ref, system)
		err = rpcServer.RegisterName("QueryReceiver", queryReceiver)
		if err != nil {
			return nil, "", err
		}
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", startPort+i+1))
		if err != nil {
			return nil, "", err
		}
		actorList = append(actorList, ref)
		go func() {
			for {
				conn, err := ln.Accept()
				if err != nil {
					return
				}
				go rpcServer.ServeConn(conn)
			}
		}()
	}

	newServer := &Server{
		actorSystem: system,
		actorList:   actorList,
	}

	// unmarshall remote representatives
	remoteServers := make([]*actor.ActorRef, 0)
	for _, remoteDesc := range remoteDescs {
		var remoteRef *actor.ActorRef
		err := json.Unmarshal([]byte(remoteDesc), &remoteRef)
		if err != nil {
			return nil, "", err

		}
		remoteServers = append(remoteServers, remoteRef)
	}
	// introduce actors in the system, and remote representatives
	for _, ref := range actorList {
		introMsg := &MIntro{ActorList: actorList, RemoteActorList: remoteServers}
		system.Tell(ref, introMsg)
	}

	actorRep := actorList[0]
	// marshal the remote representative
	marshalRef, err := json.Marshal(actorRep)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil, "", err
	}
	desc = string(marshalRef)

	// introduce representative of current actor system
	for _, remoteRef := range remoteServers {
		selfIntroMsg := MRIntro{Sender: actorRep}
		system.Tell(remoteRef, selfIntroMsg)
	}

	return newServer, desc, nil
}

// OPTIONAL: Closes the server, including its actor system
// and all RPC servers.
//
// You are not required to implement this function for full credit; the tests end
// by calling Close but do not check that it does anything. However, you
// may find it useful to implement this so that you can run multiple/repeated
// tests in the same "go test" command without cross-test interference (in
// particular, old test servers' squatting on ports.)
//
// Likewise, you may find it useful to close a partially-started server's
// resources if there is an error in NewServer.
func (server *Server) Close() {
}
