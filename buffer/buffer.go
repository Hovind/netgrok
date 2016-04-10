package buffer

import (
    "fmt"
    "time"
    . "netgrok/obj"
)

func Manager() (chan<- Message, chan<- Message, <-chan Message) {
    push_channel := make(chan Message);
    pop_channel := make(chan Message);
    
    resend_channel := make(chan Message);
    pop_success_channel := make(chan Message);

    buffer_map := make(map[int]chan<- Message)
    
    go func() {
        for {
            for k, v := range buffer_map {
                fmt.Println(k, ":", v);
            }
            select {
            case msg := <-push_channel:
                buffer_map[msg.Code] = worker(msg, resend_channel, pop_success_channel);
            case msg := <-pop_channel:
                buffer_map[msg.Code] <-msg;
            case msg := <-pop_success_channel:
                delete(buffer_map, msg.Code);
            }
        }
    }();
    return push_channel, pop_channel, resend_channel;
}

func worker(msg Message, resend_channel, pop_success_channel chan<- Message) chan<- Message {
    pop_worker_channel := make(chan Message);
    go func() {
        for {
            select {
            case <-time.After(2 * time.Second):
                resend_channel <-msg;
            case msg := <-pop_worker_channel:
                pop_success_channel <-msg;
                close(pop_worker_channel);
                return;
            }
        }   
    }();
    return pop_worker_channel;
}