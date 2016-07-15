package esgo

import (
	"errors"
	"sync"
)

var (
	InvalidCommand   = errors.New("Invalid Command, or no handler set")
	FailedStoreEvent = errors.New("Failed to store event")
	AuthFailed       = errors.New("Not authorized")
)

type CommandRouter struct {
	lock        sync.RWMutex
	cmdHandlers map[string]CommandHandler
	taskMap     map[string]TaskHandler
	es          EventStore
	auth        Auther
	authcmd     bool
}

func NewCommandRouter(es EventStore) *CommandRouter {
	var cr CommandRouter
	cr.cmdHandlers = make(map[string]CommandHandler)
	cr.taskMap = make(map[string]TaskHandler)
	cr.es = es
	return &cr
}

// Handle event into router, event handler will be executed
func (r *CommandRouter) Push(cmd *Command) *CommandResult {
	err := cmd.Validate()
	if err != nil {
		res := &CommandResult{
			Err:    InvalidCommand,
			Error:  true,
			ErrMsg: InvalidCommand.Error(),
		}
		return res
	}

	r.lock.RLock()
	h, ok := r.cmdHandlers[cmd.Name]
	r.lock.RUnlock()

	if !ok {
		res := &CommandResult{
			Err:    InvalidCommand,
			Error:  true,
			ErrMsg: InvalidCommand.Error(),
		}
		return res
	}

	if r.authcmd {
		err := r.auth.Auth(cmd)
		if err != nil {
			res := &CommandResult{
				Err:    AuthFailed,
				Error:  true,
				ErrMsg: AuthFailed.Error(),
			}
			return res
		}
	}

	event, result := h.Deal(cmd)
	if result.Error {
		return result
	}

	// store event
	sres := r.es.Store(event)
	if sres.Error != nil {
		// Failed to store event
		result.Err = FailedStoreEvent
		result.Error = true
		result.ErrMsg = sres.Error.Error()
	}

	// set event result in command response
	result.Set(sres)

	return result
}

// AddEventHandler registers event handlers into router, could be one handler for many keys
func (r *CommandRouter) AddCommandHandler(h CommandHandler, keys ...string) {
	r.lock.Lock()
	defer r.lock.Unlock()

	for _, k := range keys {
		r.cmdHandlers[k] = h
	}
}

// AddTaskHandler registers task handlers into router, could be one handler for many keys
func (r *CommandRouter) AddTaskHandler(h TaskHandler, keys ...string) {
	r.lock.Lock()
	defer r.lock.Unlock()

	for _, k := range keys {
		r.taskMap[k] = h
	}
}

func (r *CommandRouter) SetAuth(auth Auther) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if !r.authcmd {
		r.auth = auth
		r.authcmd = true
	} else {
		panic("auth already set")
	}
}
