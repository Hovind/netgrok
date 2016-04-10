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

func listening_worker(pop_channel chan<- Message, socket *net.UDPConn, local_addr *net.UDPAddr) (<-chan Packet) {
    local_addrs, _ := net.InterfaceAddrs();

    rcv_channel := make(chan Packet);

    go func() {
        b := make([]byte, 1024);
        for {
            n, addr, err := socket.ReadFromUDP(b);
            if err != nil {
                fmt.Println("Could not read from UDP:", err.Error());
            } else {
                packet := Packet{};
                err := json.Unmarshal(b[:n], &packet);
                if addr_is_remote(local_addrs, addr) && n > 0 {
                    if err != nil {
                        fmt.Println("Could not unmarshal message.");
                    } else if packet.Origin.IP.String() == local_addr.IP.String() {
                        pop_channel <-packet.Msg;
                        rcv_channel <-*NewPacket(KEEP_ALIVE, []byte{}, local_addr);

                    } else {
                        //fmt.Println("Received message with code", packet.Code, "with body", packet.Body, "from", addr.String());
                        rcv_channel <-packet;
                    }
                }
            }
        }
    }();
    return rcv_channel;
}

func send(packet Packet, socket *net.UDPConn, local_addr, addr *net.UDPAddr) (error) {
    packet.Signatures = append(packet.Signatures, local_addr.IP.String());
    b, err := json.Marshal(packet);
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
    packet := *NewPacket(HEAD_REQUEST, b, local_addr);
    return send(packet, socket, local_addr, broadcast_addr);
}

func Manager(broadcast_port string) (chan<- Message, <-chan Message) {
    broadcast_addr, _ := net.ResolveUDPAddr("udp4", net.IPv4bcast.String() + ":" + broadcast_port);
    local_addr := resolve_local_addr(broadcast_addr, broadcast_port);

    listen_addr, _ := net.ResolveUDPAddr("udp4", ":" + broadcast_port);
    socket, err := net.ListenUDP("udp4", listen_addr);
    if err != nil {
        fmt.Println("Could not create socket:", err.Error());
        return nil, nil;
    } else {
        fmt.Println("Socket has been created:", socket.LocalAddr().String());
    }

    push_channel, pop_channel, resend_channel := buffer.Manager();

    rcv_channel := listening_worker(pop_channel, socket, local_addr);

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
                case packet := <-rcv_channel:
                    switch packet.Msg.Code {
                    case TAIL_REQUEST, HEAD_REQUEST:
                        head_addr = packet.Origin;
                        b, _ := json.Marshal(packet.Origin);
                        packet := *NewPacket(CONNECTION, b, local_addr);
                        push_channel <-packet.Msg;
                        send(packet, socket, local_addr, packet.Origin);
                    case CONNECTION:
                        if err != nil {
                            fmt.Println("Could not unmarshal connection.");
                        } else {
                            head_addr = packet.Origin;
                            send(packet, socket, local_addr, head_addr);
                        }
                    }
                }
            } else {
                select {
                case msg := <-to_network_channel:
                    push_channel <-msg;
                    packet := *NewPacket(msg.Code, msg.Body, local_addr);
                    send(packet, socket, local_addr, head_addr);
                case packet :=  <-rcv_channel:
                    tail_timeout.Stop();
                    switch packet.Msg.Code {
                    case KEEP_ALIVE:
                        break;
                    case CONNECTION:
                        addr := &net.UDPAddr{};
                        err := json.Unmarshal(packet.Msg.Body, &addr);
                        if err != nil {
                            fmt.Println("Could not unmarshal connection.");
                        } else {
                            if head_addr == nil || head_addr.String() == addr.String() {
                               head_addr = packet.Origin;
                            }
                            send(packet, socket, local_addr, head_addr);
                        }
                    case HEAD_REQUEST:
                        addr := &net.UDPAddr{};
                        err := json.Unmarshal(packet.Msg.Body, &addr);
                        if err != nil {
                            fmt.Println("Could not unmarshal message.")
                        } else {
                            b, _ := json.Marshal(local_addr);
                            packet := *NewPacket(TAIL_REQUEST, b, local_addr);
                            send(packet, socket, local_addr, addr);
                        }
                    case TAIL_REQUEST:
                        break;
                    case TAIL_DEAD:
                        time.Sleep(1 * time.Second);
                        fmt.Println("Cycle broken.");
                        send(packet, socket, local_addr, head_addr);
                        head_addr = nil;
                    default:
                        from_network_channel <-packet.Msg;
                        send(packet, socket, local_addr, head_addr);
                    }
                case msg := <-resend_channel:
                    packet := *NewPacket(msg.Code, msg.Body, local_addr);
                    send(packet, socket, local_addr, head_addr);
                case <-tail_timeout.Timer.C:
                    fmt.Println("Breaking cycle.");
                    packet := *NewPacket(TAIL_DEAD, []byte{}, local_addr);
                    send(packet, socket, local_addr, head_addr);
                    head_addr = nil;
                case <-time.After(1 * time.Second):
                    packet := *NewPacket(KEEP_ALIVE, []byte{}, local_addr);
                    send(packet, socket, local_addr, head_addr);
                    if !tail_timeout.Running {
                        tail_timeout.Start(4 * time.Second);
                    }
                }
            }
        }
    }();
    return to_network_channel, from_network_channel;
}