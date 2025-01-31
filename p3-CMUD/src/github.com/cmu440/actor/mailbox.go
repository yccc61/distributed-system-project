package actor

import (
	"sync"
)

// A mailbox, i.e., a thread-safe unbounded FIFO queue.
//
// You can think of Mailbox like a Go channel with an infinite buffer.
//
// Mailbox is only exported outside of the actor package for use in tests;
// we do not expect you to use it, just implement it.
type Mailbox struct {
	queue    []any
	isClosed bool
	mux      sync.Mutex
	cond     *sync.Cond
}

// Returns a new mailbox that is ready for use.
func NewMailbox() *Mailbox {
	mailbox := &Mailbox{
		queue:    make([]any, 0),
		isClosed: false,
	}
	mailbox.cond = sync.NewCond(&mailbox.mux)
	return mailbox
}

// Pushes message onto the end of the mailbox's FIFO queue.
//
// This function should NOT block.
//
// If mailbox.Close() has already been called, this may ignore
// the message. It still should NOT block.
//
// Note: message is not a literal actor message; it is an ActorSystem
// wrapper around a marshalled actor message.
func (mailbox *Mailbox) Push(message any) {
	// TODO (3A): implement this!
	mailbox.mux.Lock()
	defer mailbox.mux.Unlock()

	// If mailbox.Close() has already been called, this may ignore
	// the message.
	if mailbox.isClosed {
		return
	}

	// Pushes message onto the end of the mailbox's FIFO queue.
	mailbox.queue = append(mailbox.queue, message)
	mailbox.cond.Signal()
}

// Pops a message from the front of the mailbox's FIFO queue,
// blocking until a message is available.
//
// If mailbox.Close() is called (either before or during a Pop() call),
// this should unblock and return (nil, false). Otherwise, it should return
// (message, true).
func (mailbox *Mailbox) Pop() (message any, ok bool) {
	mailbox.mux.Lock()
	defer mailbox.mux.Unlock()

	// if mailbox is not closed but no message in queue
	// wait for a message
	for len(mailbox.queue) == 0 && !mailbox.isClosed {
		mailbox.cond.Wait()
	}

	// If mailbox.Close() is called (either before or during a Pop() call),
	// this should unblock and return (nil, false).
	if mailbox.isClosed {
		return nil, false
	}

	message = mailbox.queue[0]
	mailbox.queue = mailbox.queue[1:]

	return message, true
}

// Closes the mailbox, causing future Pop() calls to return (nil, false)
// and terminating any goroutines running in the background.
//
// If Close() has already been called, this may exhibit undefined behavior,
// including blocking indefinitely.
func (mailbox *Mailbox) Close() {
	mailbox.mux.Lock()
	defer mailbox.mux.Unlock()

	if !mailbox.isClosed {
		mailbox.isClosed = true
		mailbox.cond.Broadcast()
	}
}
