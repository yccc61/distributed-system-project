package kvserver

import (
	"encoding/gob"
	"fmt"
	"strings"
	"time"

	"github.com/cmu440/actor"
	"github.com/cmu440/kvcommon"
)

// Implement your queryActor in this file.
// See example/counter_actor.go for an example actor using the
// github.com/cmu440/actor package.

type Content struct {
	Sender    *actor.ActorRef
	Value     string
	TimeStamp time.Time
}

type MGet struct {
	Sender *actor.ActorRef
	Key    string
}

type MPut struct {
	Sender *actor.ActorRef
	Key    string
	Value  string
}

type MList struct {
	Sender *actor.ActorRef
	Prefix string
}

type MIntro struct {
	ActorList       []*actor.ActorRef
	RemoteActorList []*actor.ActorRef
}

// recurring sync message
type MSync struct{}

type MUpdate struct {
	PutList map[string]*Content
}

type MRIntro struct {
	Sender *actor.ActorRef
}

func init() {
	gob.Register(MGet{})
	gob.Register(MPut{})
	gob.Register(MList{})
	gob.Register(MIntro{})
	gob.Register(MSync{})
	gob.Register(MUpdate{})
	gob.Register(MRIntro{})
	gob.Register(kvcommon.GetReply{})
	gob.Register(kvcommon.ListReply{})
	gob.Register(kvcommon.PutReply{})
}

type queryActor struct {
	context         *actor.ActorContext
	storage         map[string]*Content
	actorList       []*actor.ActorRef // store all the active actors, port --> actRefs
	putList         map[string]*Content
	remoteActorList []*actor.ActorRef
}

// "Constructor" for queryActors, used in ActorSystem.StartActor.
func newQueryActor(context *actor.ActorContext) actor.Actor {
	return &queryActor{
		context:         context,
		storage:         make(map[string]*Content),
		actorList:       make([]*actor.ActorRef, 0),
		putList:         make(map[string]*Content),
		remoteActorList: make([]*actor.ActorRef, 0),
	}
}

// OnMessage implements actor.Actor.OnMessage.
func (actor *queryActor) OnMessage(message any) error {
	switch m := message.(type) {
	case MGet:
		content, ok := actor.storage[m.Key]
		value := ""
		if ok {
			value = content.Value
		}
		result := kvcommon.GetReply{
			Value: value,
			Ok:    ok,
		}
		actor.context.Tell(m.Sender, result)
	case MPut:
		time := time.Now()
		actor.storage[m.Key] = &Content{Sender: m.Sender, Value: m.Value, TimeStamp: time}
		result := kvcommon.PutReply{}
		actor.context.Tell(m.Sender, result)
		actor.putList[m.Key] = &Content{Sender: m.Sender, Value: m.Value, TimeStamp: time}

	case MList:
		entries := make(map[string]string)
		for key, content := range actor.storage {
			value := content.Value
			if strings.HasPrefix(key, m.Prefix) {
				entries[key] = value
			}
		}
		result := kvcommon.ListReply{
			Entries: entries,
		}
		actor.context.Tell(m.Sender, result)

	case MIntro:
		actor.actorList = m.ActorList
		actor.remoteActorList = m.RemoteActorList
		syncMsg := &MSync{}
		actor.context.Tell(actor.context.Self, syncMsg)
		for _, ref := range m.ActorList {
			actor.context.Tell(ref, MUpdate{PutList: actor.storage})
		}

	case MSync:
		if len(actor.putList) > 0 {
			updateMsg := &MUpdate{PutList: actor.putList}
			for _, ref := range actor.actorList {
				if ref.Counter != actor.context.Self.Counter {
					actor.context.Tell(ref, updateMsg)
				}
			}
			for _, remoteRef := range actor.remoteActorList {
				actor.context.Tell(remoteRef, updateMsg)
			}
		}
		actor.putList = make(map[string]*Content)
		syncMsg := &MSync{}
		actor.context.TellAfter(actor.context.Self, syncMsg, time.Duration(250)*time.Millisecond)

	case MUpdate:
		updatedList := m.PutList
		for key, content := range updatedList {
			con, exist := actor.storage[key]
			if !exist {
				actor.storage[key] = content
				actor.putList[key] = content
			} else if con.TimeStamp.Before(content.TimeStamp) {
				actor.storage[key] = content
				actor.putList[key] = content
			} else if con.TimeStamp.Equal(content.TimeStamp) && con.Sender.Uid() < content.Sender.Uid() {
				actor.storage[key] = content
				actor.putList[key] = content
			}
		}

	case MRIntro:
		actor.remoteActorList = append(actor.remoteActorList, m.Sender)

	default:
		return fmt.Errorf("Unexpected counterActor message type: %T", m)
	}
	return nil
}
