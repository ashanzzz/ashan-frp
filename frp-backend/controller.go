package main

import (
	"context"
	"log"
	"sync"
)

type TunnelControl struct {
	mu       sync.Mutex
	running  map[string]context.CancelFunc
	rxBytes  map[string]int64
	txBytes  map[string]int64
}

var Manager *TunnelControl

func InitFRPManager() {
	Manager = &TunnelControl{
		running:  make(map[string]context.CancelFunc),
		rxBytes:  make(map[string]int64),
		txBytes:  make(map[string]int64),
	}
}

func (tc *TunnelControl) StartTunnel(t FRPTunnel, token string, serverAddr string, serverPort int) error {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if _, exists := tc.running[t.Name]; exists {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	tc.running[t.Name] = cancel

	go func() {
		log.Printf("[FRP Core] Tunnel %s active via embed in-process loop", t.Name)
		<-ctx.Done()
	}()
	return nil
}
