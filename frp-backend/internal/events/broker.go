package events

import (
    "sync"

    "ashan-frp/internal/domain"
)

type Broker struct {
    mu    sync.RWMutex
    subs  map[string]map[chan domain.Event]struct{}
}

func NewBroker() *Broker {
    return &Broker{subs: map[string]map[chan domain.Event]struct{}{}}
}

func (b *Broker) Subscribe(channel string) (<-chan domain.Event, func()) {
    if channel == "" {
        channel = "*"
    }
    ch := make(chan domain.Event, 32)

    b.mu.Lock()
    if _, ok := b.subs[channel]; !ok {
        b.subs[channel] = map[chan domain.Event]struct{}{}
    }
    b.subs[channel][ch] = struct{}{}
    b.mu.Unlock()

    unsubscribe := func() {
        b.mu.Lock()
        if subs, ok := b.subs[channel]; ok {
            delete(subs, ch)
            if len(subs) == 0 {
                delete(b.subs, channel)
            }
        }
        close(ch)
        b.mu.Unlock()
    }

    return ch, unsubscribe
}

func (b *Broker) Publish(evt domain.Event) {
    b.mu.RLock()
    defer b.mu.RUnlock()

    send := func(subs map[chan domain.Event]struct{}) {
        for ch := range subs {
            select {
            case ch <- evt:
            default:
            }
        }
    }

    if evt.Channel != "" {
        if subs, ok := b.subs[evt.Channel]; ok {
            send(subs)
        }
    }
    if subs, ok := b.subs["*"]; ok {
        send(subs)
    }
}
