package network

import (
    "fmt"
    "net"
    "strings"
    "encoding/json"
    "time"
    "netgrok/buffer"
    "netgrok/timer"
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

func resolve_local_addr(broadcast_addr *net.UDPAddr, broadcast_port string) *net.UDPAddr {
    temp_socket, _ := net.DialUDP("udp4", nil, broadcast_addr);
    defer temp_socket.Close();
    temp_addr, _ := net.ResolveUDPAddr("udp4", temp_socket.LocalAddr().String());
    local_addr, _ := net.ResolveUDPAddr("udp4", temp_addr.IP.String() + ":" + broadcast_port);
    return local_addr;
}

func listening_worker(pop_channel, sync_to_order_channel chan<- Message, socket *net.UDPConn, local_addr *net.UDPAddr) (<-chan Message) {
    local_addrs, _ := net.InterfaceAddrs();

    rcv_channel := make(chan Message);

    go func() {
        b := make([]byte, 1024);
        for {
            n, addr, err := socket.ReadFromUDP(b);
            if err != nil {
                fmt.Println("Could not read from UDP:", err.Error());
            } else {
                msg := Message{};
                err := json.Unmarshal(b[:n], &msg);
                if addr_is_remote(local_addrs, addr) && n > 0 {
                    if err != nil {
                        fmt.Println("Could not unmarshal message.");
                    } else if msg.Origin.IP.String() == local_addr.IP.String() {
                        sync_to_order_channel <-msg;
                        pop_channel <-msg;
                        rcv_channel <-*NewMessage(KEEP_ALIVE, []byte{}, local_addr, nil);

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
    b, err := json.Marshal(msg);
    if err != nil {
        fmt.Println("Could not marshal message.");
    }
    _, err = socket.WriteToUDP(b, addr);
    if err != nil {
        fmt.Println("Could not send:", err.Error());
    } else {
        //fmt.Println("Sent message with code", msg.Code, "and body", msg.Body, "to:", addr.String());
    }
    return err;
}

func request_head(local_addr, broadcast_addr *net.UDPAddr, socket *net.UDPConn) (error) {
    b, err := json.Marshal(local_addr);
    if err != nil {
        fmt.Println("Could not marshal local address");
        return err;
    }
    msg := *NewMessage(HEAD_REQUEST, b, local_addr, nil);
    return send(msg, socket, broadcast_addr);
}

func Manager(broadcast_port string) (chan<- Message, <-chan Message, <-chan Message, <-chan chan Message) {
    broadcast_addr, _ := net.ResolveUDPAddr("udp4", net.IPv4bcast.String() + ":" + broadcast_port);
    local_addr := resolve_local_addr(broadcast_addr, broadcast_port);

    listen_addr, _ := net.ResolveUDPAddr("udp4", ":" + broadcast_port);
    socket, err := net.ListenUDP("udp4", listen_addr);
    if err != nil {
        fmt.Println("Could not create socket:", err.Error());
        return nil, nil, nil, nil;
    } else {
        fmt.Println("Socket has been created:", socket.LocalAddr().String());
    }

    push_channel, pop_channel, resend_channel := buffer.Manager();

    sync_to_order_channel := make(chan Message);
    sync_request_channel := make(chan chan Message);
    rcv_channel := listening_worker(pop_channel, sync_to_order_channel, socket, local_addr);

    to_network_channel := make(chan Message);
    from_network_channel := make(chan Message);
    go func() {
        head_addr := (*net.UDPAddr)(nil);
        tail_timeout := timer.New();
        for {
            if head_addr == nil {
                request_head(local_addr, broadcast_addr, socket);
                select {
                case <-time.After(10 * time.Second):
                    continue;
                case msg := <-rcv_channel:
                    switch msg.Code {
                    case TAIL_REQUEST, HEAD_REQUEST:
                        head_addr = msg.Origin;
                        msg := *NewMessage(CONNECTION, []byte{}, local_addr, msg.Origin);
                        push_channel <-msg;
                        send(msg, socket, msg.Origin);
                    case CONNECTION:
                        head_addr = msg.Origin;
                        send(msg, socket, head_addr);
                    }
                }
            } else {
                select {
                case msg := <-to_network_channel:
                    msg.Origin = local_addr;
                    push_channel <-msg;
                    send(msg, socket, head_addr);
                case msg :=  <-rcv_channel:
                    tail_timeout.Stop();
                    switch msg.Code {
                    case KEEP_ALIVE:
                        break;
                    case CONNECTION:
                        if head_addr == nil || head_addr.String() == msg.Target.String() {
                           head_addr = msg.Origin;
                        }
                        send(msg, socket, head_addr);
                    case HEAD_REQUEST:
                        msg := *NewMessage(TAIL_REQUEST, []byte{}, local_addr, nil);
                        send(msg, socket, msg.Origin);
                    case TAIL_REQUEST:
                        break;
                    case TAIL_DEAD:
                        time.Sleep(1 * time.Second);
                        fmt.Println("Cycle broken.");
                        send(msg, socket, head_addr);
                        head_addr = nil;
                    case SYNC_CART:
                        sync_to_order_channel <-msg;
                        sync_from_order_channel := make(chan Message);
                        sync_request_channel <-sync_from_order_channel;
                        msg := <-sync_from_order_channel;
                        close(sync_from_order_channel);
                        send(msg, socket, head_addr);
                    default:
                        from_network_channel <-msg;
                        send(msg, socket, head_addr);
                    }
                case msg := <-resend_channel:
                    send(msg, socket, head_addr);
                case <-tail_timeout.Timer.C:
                    fmt.Println("Breaking cycle.");
                    msg := *NewMessage(TAIL_DEAD, nil, local_addr, nil);
                    send(msg, socket, head_addr);
                    head_addr = nil;
                case <-time.After(1 * time.Second):
                    msg := *NewMessage(KEEP_ALIVE, nil, local_addr, nil);
                    send(msg, socket, head_addr);
                    if !tail_timeout.Running {
                        tail_timeout.Start(4 * time.Second);
                    }
                }
            }
        }
    }();
    return to_network_channel, from_network_channel, sync_to_order_channel, sync_request_channel;
}
