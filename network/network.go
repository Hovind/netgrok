package network

import (
    "fmt"
    "net"
    "strconv"
    "encoding/json"
    "time"
    "netgrok/buffer"
    . "netgrok/obj"
)


func listener(socket *net.UDPConn) (<-chan Message) {
    out := make(chan Message);
    go func () {

        b := make([]byte, 1024);
        for {
            n, addr, err := socket.ReadFromUDP(b);
            if err != nil {
                return;
            }
            if addr.String() != local_addr.String() && n > 0 {
                msg := Message{};
                err := json.Unmarshal(b[:n], &msg);
                if err != nil {
                    fmt.Println("Could not unmarshal message.");
                } else {
                    out <-msg;
                }
                //fmt.Println("Received message with code", msg.Code, "with body", msg.Body, "from", addr.String());

            }
        }
    }();
    return out;
}

func listening_manager(pop_channel chan<- Message, socket, broadcast_socket *net.UDPConn) (<-chan Message) {
    broadcast_in_channel := listener(broadcast_socket);
    tail_in_channel := listener(socket);

    out := make(chan Message);
    
    go func() {
        for {
            select {
            case msg := <-broadcast_in_channel:
                out <-msg;
            case msg := <-tail_in_channel:
                if /*len(msg.Signatures) != 0 && */msg.Signatures[0] == local_addr.String() {
                    pop_channel <-msg;
                } else {
                    out <-msg;
                }
            }
        }
    }();
    return out;
}

func send(msg Message, addr *net.UDPAddr) (error) {
    msg.Signatures = append(msg.Signatures, local_addr.String());
    b, err := json.Marshal(msg);
    if err != nil {
        fmt.Println("Could not marshal message.");
    }
    _, err = socket.WriteToUDP(b, addr);
    if err != nil {
        fmt.Println("Could not send!");
    } else {
        //fmt.Println("Sent message with code", msg.Code, "and body", msg.Body, "to:", addr.String());
    }
    return err;
}

func sending_manager(push_channel chan<- Message) (chan<- Message, chan<- Message, chan<- Message, chan<- *net.UDPAddr) {
    send_channel := make(chan Message);
    relay_channel := make(chan Message);
    broadcast_channel := make(chan Message);
    tail_channel := make(chan *net.UDPAddr);

    go func() {
        for {
            select {
            case msg := <-send_channel:
                push_channel <-msg;
                send(msg, head_addr);
            case msg := <-relay_channel:
                send(msg, head_addr);
            case msg := <-broadcast_channel:
                send(msg, broadcast_addr);
            case addr := <-tail_channel:
                b, _ := json.Marshal(local_addr);
                msg := NewMessage(TAIL_REQUEST, b);
                send(*msg, addr);
            }
        }
    }();
    return send_channel, relay_channel, broadcast_channel, tail_channel;
}

var local_addr, head_addr, broadcast_addr *net.UDPAddr;
var socket, broadcast_socket *net.UDPConn;

func Init(in_port, broadcast_in_port string) (chan<- Message, <-chan Message) {
    broadcast_addr, _ = net.ResolveUDPAddr("udp4", "255.255.255.255" + ":" + broadcast_in_port);

    temp_socket, err := net.DialUDP("udp4", nil, broadcast_addr);
    defer temp_socket.Close();
    temp_addr := temp_socket.LocalAddr();
    local_addr, err = net.ResolveUDPAddr("udp4", temp_addr.String());
    local_addr.Port, _ = strconv.Atoi(in_port);

    socket, _ = net.ListenUDP("udp4", local_addr);
    if err != nil {
        fmt.Println("Could not create socket.");
        return nil, nil;
    }
    broadcast_socket, _ := net.ListenUDP("udp", broadcast_addr);
    if err != nil {
        fmt.Println("Could not create broadcast socket.");
        socket.Close();
        return nil, nil;

    } else {
        fmt.Println("Sockets have been created.");
    }

    var tail_timeout_channel/*, cycle_timeout_channel*/ <-chan time.Time;

    push_channel, pop_channel := buffer.Init();
    rcv_channel := listening_manager(pop_channel, socket, broadcast_socket);
    send_channel, relay_channel, broadcast_channel, tail_channel := sending_manager(push_channel);

    to_network_channel := make(chan Message);
    from_network_channel := make(chan Message);
    go func() {
        for {
            if head_addr == nil {
                b, _ := json.Marshal(local_addr)
                broadcast_channel <-*NewMessage(HEAD_REQUEST, b);
                select {
                case <- time.After(4 * time.Second):
                    continue;
                case msg := <-rcv_channel:
                    switch msg.Code {
                    case TAIL_REQUEST:
                        var addr *net.UDPAddr;
                        _ = json.Unmarshal(msg.Body, &addr);
                        head_addr = addr;
                        conn := NewConnection(local_addr, addr);
                        b, _ := json.Marshal(conn);
                        send_channel <-*NewMessage(CONNECTION, b);
                    case HEAD_REQUEST:
                        var addr *net.UDPAddr;
                        _ = json.Unmarshal(msg.Body, &addr);
                        tail_channel <-addr;
                    }
                }
            } else {
                select {
                case msg := <-to_network_channel:
                    send_channel <-msg;
                case msg :=  <-rcv_channel:
                    switch msg.Code {
                    case KEEP_ALIVE:
                        relay_channel <-msg;
                    case CONNECTION:
                        var conn Connection;
                        err := json.Unmarshal(msg.Body, &conn);
                        if err != nil {
                            fmt.Println("Could not unmarshal connection.");
                        } else {
                            if head_addr == nil || head_addr.String() == conn.To.String() {
                               head_addr = conn.From;
                            }
                            relay_channel <-msg;
                        }
                    case HEAD_REQUEST:
                        var addr *net.UDPAddr;
                        err := json.Unmarshal(msg.Body, &addr);
                        if err != nil {
                            fmt.Println("Could not unmarshal message.")
                        } else {
                            tail_channel <-addr;
                        }
                    case TAIL_REQUEST:
                        break;
                    case TAIL_DEAD:
                        time.Sleep(1 * time.Second);
                        relay_channel <-msg;
                        head_addr = nil;
                    default:
                        from_network_channel <-msg;
                        relay_channel <-msg;
                    }
                    tail_timeout_channel = nil;
                case <- time.After(1 * time.Second):
                    send_channel <-*NewMessage(KEEP_ALIVE, []byte{});
                    if tail_timeout_channel == nil {
                        tail_timeout_channel = time.After(2 * time.Second);
                    }
                case <-tail_timeout_channel:
                    send_channel <-*NewMessage(TAIL_DEAD, []byte{});
                    head_addr = nil;
                }
            }
        }
    }();
    return to_network_channel, from_network_channel;
}