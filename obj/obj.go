package network

import (
    "net"
)

type Message struct {
    Code int
    Body []byte
    Origin, Target *net.UDPAddr
}

type Packet struct {
    Msg Message
}


func NewMessage(code int, body []byte, local_addr, target_addr *net.UDPAddr) *Message {
    return &Message{Code: code, Body: body, Origin: local_addr, Target: target_addr};
}


func (msg *Message) Hash() int {
    hash := msg.Code;
    for _, e := range msg.Body {
        hash = 2*hash ^ int(e);
    }
    return hash;
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
