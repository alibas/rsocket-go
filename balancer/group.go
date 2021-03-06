package balancer

import (
	"errors"
	"sync"

	"github.com/rsocket/rsocket-go/logger"
)

var errGroupClosed = errors.New("balancer group has been closed")

// Group manage a group of Balancer.
// Group can be used to create a simple RSocket Broker.
type Group struct {
	g func() Balancer
	m *sync.Map
}

// Close close current RSocket group.
func (p *Group) Close() (err error) {
	if p.m == nil {
		return
	}
	all := make(chan Balancer)
	done := make(chan struct{})
	go func(all chan Balancer, done chan struct{}) {
		defer func() {
			close(done)
		}()
		for it := range all {
			if err := it.Close(); err != nil {
				logger.Warnf("close balancer failed: %s\n", err)
			}
		}
	}(all, done)
	p.m.Range(func(key, value interface{}) bool {
		all <- value.(Balancer)
		return true
	})
	p.m = nil
	close(all)
	<-done
	return
}

// Get returns a Balancer with custom id.
func (p *Group) Get(id string) Balancer {
	if p.m == nil {
		panic(errGroupClosed)
	}
	if actual, ok := p.m.Load(id); ok {
		return actual.(Balancer)
	}
	newborn := p.g()
	actual, loaded := p.m.LoadOrStore(id, newborn)
	if loaded {
		_ = newborn.Close()
	}
	return actual.(Balancer)
}

// NewGroup returns a new Group.
func NewGroup(gen func() Balancer) *Group {
	return &Group{
		g: gen,
		m: &sync.Map{},
	}
}
