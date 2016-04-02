package network

import (
    "net"
)

type Message struct {
    Code int
    Body []byte
    Signatures []string
}
type Connection struct {
    From, To *net.UDPAddr 
}

func NewMessage(code int, body []byte) *Message {
    return &Message{Code: code, Body: body, Signatures: []string{}};
}
func NewConnection(from, to *net.UDPAddr) *Connection {
    return &Connection{From: from, To: to};
}

const (
    ORDER_PUSH = 100 + iota
    ORDER_POP
    FLOOR_HIT
    DIRECTION_CHANGE
    SYNC_CART
)

const (
    KEEP_ALIVE = 200 + iota
)

const (
    CONNECTION = 300 + iota
    HEAD_REQUEST
    TAIL_REQUEST
)

const (
    TAIL_DEAD = 400 + iota
    CYCLE_BREAK
)