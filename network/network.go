package network

import (
    "net"
    "strings"
    "encoding/json"
    "time"
    "netgrok/buffer"
    . "netgrok/obj"
)


func listener(pop_channel chan<- Message) (<-chan Message) {
    out := make(chan Message);
    go func () {
        sock, err := net.ListenUDP("udp", in_addr);
        if err != nil {
            fmt.fatal("Could not open listening socket.");
        }
        defer sock.Close();
        
        var buffer [1024]byte;
        for {
            _, _, err = sock.ReadFromUDP(buffer[:]);
            if err != nil {
                return;
            }
            var obj Message;
            _ = json.Unmarshal(buffer[:], obj);
            if obj.Signatures[0] == in_addr.String() {
                pop_channel <- obj;
            } else {
                out <- obj;
            }
        }
    }();
    return out;
}

func speaker(push_channel chan<- Message) (chan<- Message, chan<- Message, chan<- *net.UDPAddr) {
    send_channel := make(chan Message);
    broadcast_channel := make(chan Message);
    connection_channel := make(chan *net.UDPAddr);

    go func() {
        for {
            select {
            case msg := <- send_channel:
                push_channel <-msg;
                msg.Signatures = append(msg.Signatures, in_addr.String());
                send(msg, head_socket);
            case msg := <-broadcast_channel:
                send(msg, broadcast_socket);
            case addr := <-connection_channel:
                head_socket, _ = net.DialUDP("udp", out_addr, addr);

                conn := NewConnection(in_addr, addr);
                b, _ := json.Marshal(conn);
                msg := NewMessage(CONNECTION, b);
                send(*msg, head_socket);
            }
        }
    }();
    return send_channel, broadcast_channel, connection_channel;
}

func send(msg Message, socket *net.UDPConn) (error) {
    b, _ := json.Marshal(msg);
    _, err := socket.Write(b);
    return err;
}
/*
func broadcast(obj Message) (error) {
    obj := Message {
        Code:   code,
        Body: []byte(msg),
        Signatures: []string{},
    }

    payload, _ := json.Marshal(obj);

    _, err = broadcast_socket.Write(payload);
    return err;
}

func connect(addr *net.UDPAddr) (error) {
    head_socket, err = net.DialUDP("udp", out_addr, addr);
    obj := Message {
        Code:   CONNECTION,
        Body: json.Marshal(addr),
        Signatures: []string{},
    }
    send(obj);
    return err;
}*/

var in_addr, out_addr *net.UDPAddr;
var broadcast_socket, head_socket *net.UDPConn;

func Init(in_port, out_port string) (chan Message) {
    local_ips, _ := net.InterfaceAddrs();
    local_ip := strings.Split(local_ips[1].String(), "/")[0];

    in_addr, _ = net.ResolveUDPAddr("udp", local_ip + ":" + in_port);
    out_addr, _ = net.ResolveUDPAddr("udp", local_ip + ":" + out_port);

    broadcast_addr := &net.UDPAddr {
        IP: make([]byte, 16, 16),
        Port: out_addr.Port,
        Zone: out_addr.Zone };
    copy(broadcast_addr.IP, out_addr.IP);
    broadcast_addr.IP[15] = 255;

    /*fmt.Println(in_addr);
    fmt.Println(out_addr);
    fmt.Println(broadcast_addr);*/

    broadcast_socket, _ = net.DialUDP("udp", out_addr, broadcast_addr);

    var tail_timeout_channel/*, cycle_timeout_channel*/ <-chan time.Time;

    push_channel, pop_channel := buffer.Init();
    rcv_channel := listener(pop_channel);
    send_channel, broadcast_channel, connection_channel := speaker(push_channel);
 
    message_channel := make(chan Message);
    go func() {
        for {
            if head_socket == nil {
                head_request := *NewMessage(HEAD_REQUEST, []byte{});
                broadcast_channel <-head_request;
                select {
                case <- time.After(5 * time.Second):
                    continue;
                case msg := <-rcv_channel:
                    if msg.Code == TAIL_REQUEST {
                        var addr *net.UDPAddr;
                        _ = json.Unmarshal(msg.Body[:], addr);
                        connection_channel <-addr;
                    }
                }
            } else {
                select {
                case <- time.After(4):
                    send_channel <-*NewMessage(KEEP_ALIVE, []byte{});
                    tail_timeout_channel = time.After(8);
                case msg :=  <-rcv_channel:
                    switch msg.Code {
                    case KEEP_ALIVE:
                        break;
                    case CONNECTION:
                        var conn Connection;
                        _ = json.Unmarshal(msg.Body[:], conn);
                        if head_socket == nil || head_socket.RemoteAddr() == conn.To {
                            connection_channel <-conn.From;
                        } else {
                            send_channel <- msg;
                        }
                    case HEAD_REQUEST:
                    case TAIL_REQUEST:
                    case TAIL_DEAD:
                        time.Sleep(1 * time.Second);
                    default:
                        message_channel <-msg;
                    }
                    tail_timeout_channel = nil;
                case <- tail_timeout_channel:
                    send_channel <-*NewMessage(TAIL_DEAD, []byte{});
                    head_socket = nil;
                }
            }
        } 
    }();
    return message_channel;
}