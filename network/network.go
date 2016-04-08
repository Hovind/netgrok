package network

import (
    "fmt"
    "net"
    "strings"
    "encoding/json"
    "time"
    "netgrok/buffer"
    . "netgrok/obj"
)

func addr_is_remote(local_addrs []net.Addr, addr *net.UDPAddr) bool {
    for _, local_addr := range local_addrs {
        if strings.Contains(local_addr.String(), addr.IP.String()) {
            return false;
        }
    }
    return true;
}

func listening_worker(pop_channel chan<- Message, local_addr *net.UDPAddr) (<-chan Message) {
    local_addrs, err := net.InterfaceAddrs();
    tail_socket, err := net.ListenUDP("udp4", local_addr);

    if err != nil {
        fmt.Println("Could not create tail socket.");
        return nil;
    }

    addr := (*net.UDPAddr)(nil);
    rcv_channel := make(chan Message);
    go func() {
        b := make([]byte, 1024);
        for {
            var n int;
            var err error;
            n, addr, err = tail_socket.ReadFromUDP(b);
            if err != nil {
                fmt.Println("Could not read from UDP!");
            } else {
                if addr_is_remote(local_addrs, addr) && n > 0 {
                    msg := Message{};
                    err := json.Unmarshal(b[:n], &msg);
                    if err != nil {
                        fmt.Println("Could not unmarshal message.");
                    } else if msg.Signatures[0] == local_addr.IP.String() {
                        fmt.Println("Received message with code", msg.Code, "with body", msg.Body, "from", addr.String());
                        pop_channel <-msg;
                    } else {
                        rcv_channel <-msg;
                    }
                }
            }
        }
    }();
    return rcv_channel;
}

func send(msg Message, socket *net.UDPConn, addr *net.UDPAddr) (error) {
    msg.Signatures = append(msg.Signatures, socket.LocalAddr().String());
    fmt.Println("Ready to send", msg);
    b, err := json.Marshal(msg);
    if err != nil {
        fmt.Println("Could not marshal message.");
    }
    _, err = socket.WriteToUDP(b, addr);
    if err != nil {
        fmt.Println(err.Error())
        fmt.Println("Could not send!");
    } else {
        fmt.Println("Sent message with code", msg.Code, "and body", msg.Body, "to:", addr.String());
    }
    return err;
}

func request_head(local_addr *net.UDPAddr, head_socket *net.UDPConn) (error) {
    b, err := json.Marshal(local_addr);
    if err != nil {
        fmt.Println("Could not marshal local address");
        return err;
    }
    msg := *NewMessage(HEAD_REQUEST, b);
    msg.Signatures = append(msg.Signatures, local_addr.String());
    b, err = json.Marshal(msg);
    if err != nil {
        fmt.Println("Could not marshal message.");
        return err;
    }
    _, err = head_socket.Write(b);
    if err != nil {
        fmt.Println("Could not send!");
    } else {
        //fmt.Println("Sent message with code", msg.Code, "and body", msg.Body, "to:", addr.String());
    }
    return err;
}

func Manager(broadcast_port string) (chan<- Message, <-chan Message) {
    broadcast_addr, _ := net.ResolveUDPAddr("udp4", net.IPv4bcast.String() + ":" + broadcast_port);

    temp_socket, _ := net.DialUDP("udp4", nil, broadcast_addr);
    local_addr, _ := net.ResolveUDPAddr("udp4", temp_socket.LocalAddr().String());
    temp_socket.Close();
    
    temp_socket = nil;

    push_channel, pop_channel := buffer.Manager();
    rcv_channel := listening_worker(pop_channel, local_addr);

    to_network_channel := make(chan Message);
    from_network_channel := make(chan Message);
    go func() {
        head_addr := (*net.UDPAddr)(nil);
        fmt.Println(local_addr);
        head_socket, err := net.ListenUDP("udp4", local_addr);
        if err != nil {
            fmt.Println("Could not create head socket.");
            return;
        } else {
            fmt.Println("Head socket has been created.");
        }
        defer head_socket.Close();
        tail_timeout := (*time.Timer)(nil);

        for {
            if head_addr == nil {
                request_head(local_addr, head_socket);
                select {
                case <-time.After(10 * time.Second):
                    continue;
                case msg := <-rcv_channel:
                    switch msg.Code {
                    case TAIL_REQUEST:
                        addr := &net.UDPAddr{};
                        err := json.Unmarshal(msg.Body, &addr);
                        if err != nil {
                            fmt.Println("Could not unmarshal address.");
                        } else {
                            head_addr = addr;
                            conn := NewConnection(local_addr, addr);
                            b, _ := json.Marshal(conn);
                            msg := *NewMessage(CONNECTION, b);
                            push_channel <-msg;
                            send(msg, head_socket, addr);
                        }
                    case HEAD_REQUEST:
                        //SPAWN CONNECTION
                        addr := &net.UDPAddr{};
                        err := json.Unmarshal(msg.Body, &addr);
                        if err != nil {
                            fmt.Println("Could not unmarshal message.")
                        } else {
                            b, _ := json.Marshal(local_addr);
                            msg := *NewMessage(TAIL_REQUEST, b);
                            send(msg, head_socket, addr);
                        }
                    }
                }
            } else {
                select {
                case msg := <-to_network_channel:
                    push_channel <-msg;
                    send(msg, head_socket, head_addr);
                case msg :=  <-rcv_channel:
                    tail_timeout = nil;
                    switch msg.Code {
                    case KEEP_ALIVE:
                        break;
                    case CONNECTION:
                        var conn Connection;
                        err := json.Unmarshal(msg.Body, &conn);
                        if err != nil {
                            fmt.Println("Could not unmarshal connection.");
                        } else {
                            if head_addr == nil || head_addr.String() == conn.To.String() {
                               head_addr = conn.From;
                            }
                            send(msg, head_socket, head_addr);
                        }
                    case HEAD_REQUEST:
                        addr := &net.UDPAddr{};
                        err := json.Unmarshal(msg.Body, &addr);
                        if err != nil {
                            fmt.Println("Could not unmarshal message.")
                        } else {
                            b, _ := json.Marshal(local_addr);
                            msg := *NewMessage(TAIL_REQUEST, b);
                            send(msg, head_socket, addr);
                        }
                    case TAIL_REQUEST:
                        break;
                    case TAIL_DEAD:
                        time.Sleep(1 * time.Second);
                        fmt.Println("Cycle broken.");
                        send(msg, head_socket, head_addr);
                        head_addr = nil;
                    default:
                        from_network_channel <-msg;
                        send(msg, head_socket, head_addr);
                    }
                case <-time.After(1 * time.Second):
                    msg := *NewMessage(KEEP_ALIVE, []byte{});
                    send(msg, head_socket, head_addr);
                    tail_timeout = time.NewTimer(500 * time.Millisecond);
                case <-tail_timeout.C:
                    fmt.Println("Breaking cycle.");
                    msg := *NewMessage(TAIL_DEAD, []byte{});
                    send(msg, head_socket, head_addr);
                    head_addr = nil;
                    tail_timeout = nil;
                }
            }
        }
    }();
    return to_network_channel, from_network_channel;
}