package callback

import (
	m "github.com/linkedin-inc/mane/model"
	"sync"

	"github.com/go-errors/errors"
)

var (
	ErrDuplicatedCallback = errors.New("duplicated callback")
	ErrCallbackNotFound   = errors.New("callback not found")
)

type Name string

type Callback func(status *m.DeliveryStatus, history *m.SMSHistory) error

type callRegistry struct {
	//use a mutex to avoid duplicated register
	locker    *sync.RWMutex
	callbacks map[Name]Callback
}

var registry callRegistry

func init() {
	registry = callRegistry{
		locker:    new(sync.RWMutex),
		callbacks: make(map[Name]Callback),
	}
	RegisterAll()
}

//Register callback with given name
func Register(name Name, callback Callback) error {
	registry.locker.Lock()
	defer registry.locker.Unlock()
	_, existed := registry.callbacks[name]
	if existed {
		return ErrDuplicatedCallback
	}
	registry.callbacks[name] = callback
	return nil
}

//Lookup return the callback for given name
func Lookup(name Name) (Callback, error) {
	if name == "" {
		//assume callback not set
		return nil, nil
	}
	registry.locker.RLock()
	defer registry.locker.RUnlock()
	callback, existed := registry.callbacks[name]
	if !existed {
		return nil, ErrCallbackNotFound
	}
	return callback, nil
}
