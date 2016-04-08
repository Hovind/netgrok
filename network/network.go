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

func listening_worker(pop_channel chan<- Message, local_addr *net.UDPAddr, broadcast_port string) (<-chan Message) {
    local_addrs, err := net.InterfaceAddrs();
    tail_addr, _ := net.ResolveUDPAddr("udp4", ":" + broadcast_port)
    tail_socket, err := net.ListenUDP("udp4", tail_addr);

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
                        pop_channel <-msg;
                    } else {
                        //fmt.Println("Received message with code", msg.Code, "with body", msg.Body, "from", addr.String());
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
    b, err := json.Marshal(msg);
    if err != nil {
        fmt.Println("Could not marshal message.");
    }
    _, err = socket.WriteToUDP(b, addr);
    if err != nil {
        fmt.Println("Could not send!");
    } else {
        fmt.Println("Sent message with code", msg.Code, "and body", msg.Body, "to:", addr.String());
    }
    return err;
}

func request_head(local_addr, broadcast_addr *net.UDPAddr, head_socket *net.UDPConn) (error) {
    b, err := json.Marshal(local_addr);
    if err != nil {
        fmt.Println("Could not marshal local address");
        return err;
    }
    msg := *NewMessage(HEAD_REQUEST, b);
    return send(msg, head_socket, broadcast_addr);
}

func Manager(broadcast_port string) (chan<- Message, <-chan Message) {
    broadcast_addr, _ := net.ResolveUDPAddr("udp4", net.IPv4bcast.String() + ":" + broadcast_port);

    temp_socket, _ := net.DialUDP("udp4", nil, broadcast_addr);
    local_addr, _ := net.ResolveUDPAddr("udp4", temp_socket.LocalAddr().String());
    temp_socket.Close();

    push_channel, pop_channel := buffer.Manager();
    rcv_channel := listening_worker(pop_channel, local_addr, broadcast_port);

    to_network_channel := make(chan Message);
    from_network_channel := make(chan Message);
    go func() {
        head_addr := (*net.UDPAddr)(nil);
        fmt.Println(local_addr);
        head_socket, err := net.ListenUDP("udp4", local_addr);
        if err != nil {
            fmt.Println("Could not create head socket.", err.Error());
            return;
        } else {
            fmt.Println("Head socket has been created.");
        }
        defer head_socket.Close();
        tail_timeout := time.NewTimer(0 * time.Second);
        <-tail_timeout.C;

        for {
            fmt.Println("HEAD:", head_addr);
            if head_addr == nil {
                request_head(local_addr, broadcast_addr, head_socket);
                select {
                case <-time.After(10 * time.Second):
                    continue;
                case msg := <-rcv_channel:
                    switch msg.Code {
                    case TAIL_REQUEST, HEAD_REQUEST:
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
                    case CONNECTION:
                        var conn Connection;
                        err := json.Unmarshal(msg.Body, &conn);
                        if err != nil {
                            fmt.Println("Could not unmarshal connection.");
                        } else {
                            head_addr = conn.From;
                            send(msg, head_socket, head_addr);
                        }
                    /*case HEAD_REQUEST:
                        //SPAWN CONNECTION
                        addr := &net.UDPAddr{};
                        err := json.Unmarshal(msg.Body, &addr);
                        fmt.Println("Got head request from", addr);
                        if err != nil {
                            fmt.Println("Could not unmarshal message.")
                        } else {
                            b, _ := json.Marshal(local_addr);
                            msg := *NewMessage(TAIL_REQUEST, b);
                            send(msg, head_socket, addr);
                        }*/
                    }
                }
            } else {
                select {
                case msg := <-to_network_channel:
                    push_channel <-msg;
                    send(msg, head_socket, head_addr);
                case msg :=  <-rcv_channel:
                    tail_timeout.Stop();
                    switch msg.Code {
                    case KEEP_ALIVE:
                        fmt.Println("KEPT ALIVE");
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
                case <-tail_timeout.C:
                    fmt.Println("Breaking cycle.");
                    msg := *NewMessage(TAIL_DEAD, []byte{});
                    send(msg, head_socket, head_addr);
                    head_addr = nil;
                case <-time.After(1 * time.Second):
                    msg := *NewMessage(KEEP_ALIVE, []byte{});
                    send(msg, head_socket, head_addr);
                    tail_timeout.Reset(900 * time.Millisecond);

                }
            }
        }
    }();
    return to_network_channel, from_network_channel;
}