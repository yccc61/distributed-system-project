package actor

import (
	"net/rpc"
)

// args
type RemoteMessage struct {
	Mars []byte
	Ref  *ActorRef
}

// reply
type Reply struct{}

// Calls system.tellFromRemote(ref, mars) on the remote ActorSystem listening
// on ref.Address.
//
// This function should NOT wait for a reply from the remote system before
// returning, to allow sending multiple messages in a row more quickly.
// It should ensure that messages are delivered in-order to the remote system.
// (You may assume that remoteTell is not called multiple times
// concurrently with the same ref.Address).
func remoteTell(client *rpc.Client, ref *ActorRef, mars []byte) {
	// TODO (3B): implement this!
	// call the actorsystem.remotetell with client.call
	// helper: RPC handler
	client.Go("RemoteTellHandler.RemoteTell", &RemoteMessage{Mars: mars, Ref: ref}, &Reply{}, nil)
}

// Registers an RPC handler on server for remoteTell calls to system.
//
// You do not need to start the server's listening on the network;
// just register a handler struct that handles remoteTell RPCs by calling
// system.tellFromRemote(ref, mars).
func registerRemoteTells(system *ActorSystem, server *rpc.Server) error {
	// TODO (3B): implement this!
	// register the name space of the system/rpc handler
	handler := &RemoteTellHandler{
		system: system,
	}
	err := server.RegisterName("RemoteTellHandler", handler)
	return err
}

// TODO (3B): implement your remoteTell RPC handler below!
// remote tell procedure, call tellFromRemote
type RemoteTellHandler struct {
	system *ActorSystem
}

func (handler RemoteTellHandler) RemoteTell(msg *RemoteMessage, reply *Reply) error {
	handler.system.tellFromRemote(msg.Ref, msg.Mars)
	return nil
}
