package kvserver

import (
	"github.com/cmu440/actor"
	"github.com/cmu440/kvcommon"
)

// RPC handler implementing the kvcommon.QueryReceiver interface.
// There is one queryReceiver per queryActor, each running on its own port,
// created and registered for RPCs in NewServer.
//
// A queryReceiver MUST answer RPCs by sending a message to its query
// actor and getting a response message from that query actor (via
// ActorSystem's NewChannelRef). It must NOT attempt to answer queries
// using its own state, and it must NOT directly coordinate with other
// queryReceivers - all coordination is done within the actor system
// by its query actor.
type queryReceiver struct {
	// TODO (3A): implement this!
	ref    *actor.ActorRef
	system *actor.ActorSystem
}

func NewQueryReceiver(ref *actor.ActorRef, system *actor.ActorSystem) *queryReceiver {
	return &queryReceiver{
		ref:    ref,
		system: system,
	}
}

// Get implements kvcommon.QueryReceiver.Get.
func (rcvr *queryReceiver) Get(args kvcommon.GetArgs, reply *kvcommon.GetReply) error {
	// TODO (3A): implement this!
	chanRef, respCh := rcvr.system.NewChannelRef()
	msg := MGet{
		Sender: chanRef,
		Key:    args.Key,
	}

	// ref := rcvr.actorSystem.StartActor(newQueryActor)
	rcvr.system.Tell(rcvr.ref, msg)

	response := (<-respCh).(kvcommon.GetReply)

	value, ok := response.Value, response.Ok

	reply.Value = value
	reply.Ok = ok
	return nil
}

// List implements kvcommon.QueryReceiver.List.
func (rcvr *queryReceiver) List(args kvcommon.ListArgs, reply *kvcommon.ListReply) error {
	// TODO (3A): implement this!
	chanRef, respCh := rcvr.system.NewChannelRef()
	msg := MList{
		Sender: chanRef,
		Prefix: args.Prefix,
	}

	rcvr.system.Tell(rcvr.ref, msg)

	response := (<-respCh).(kvcommon.ListReply)

	res := response.Entries

	reply.Entries = res
	return nil
}

// Put implements kvcommon.QueryReceiver.Put.
func (rcvr *queryReceiver) Put(args kvcommon.PutArgs, reply *kvcommon.PutReply) error {
	// TODO (3A): implement this!
	chanRef, respCh := rcvr.system.NewChannelRef()
	msg := MPut{
		Sender: chanRef,
		Key:    args.Key,
		Value:  args.Value,
	}
	rcvr.system.Tell(rcvr.ref, msg)
	response := (<-respCh).(kvcommon.PutReply)
	reply = &response
	return nil
}
