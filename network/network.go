package network

import (
    "os"
    "fmt"
    "net"
    "strings"
    "strconv"
    "encoding/json"
    "time"
    "bytes"
    "buffer"
)


func listener(pop_channel chan<- Message) (<-chan Message) {
    out := make(chan Message);
    go func () {
        sock, err := net.ListenUDP("udp", in_addr);
        defer sock.Close();
        var buffer [1024]byte;
        for {
            _, _, err = sock.ReadFromUDP(buffer[:]);
            if err != nil {
                return;
            }
            var obj Message;
            _ = json.Unmarshal(buffer[:], obj);
            if obj.Signatures[0] == local_ip {
                pop_channel <- obj;
            } else {
                out <- obj;
            }
        }
    }();
    return out;
}

func speaker(push_channel chan<- Message) (chan<- Message, chan<- *net.UDPAddr) {
    send_channel := make(chan Message);
    connection_channel := make(chan *net.UDPAddr);    

    go func() {
        var head_socket := nil;
        for {
            select {
            case obj := <- send_channel:
                push_channel <-obj;
                obj.Signatures = append(obj.Signatures, in_addr.String());
                b := json.Marshal(obj);
                _, err := head_socket.Write(b);
            case obj := <-broadcast_channel:
                b := json.Marshal(obj);
                _, err := head_socket.Write(b);
            case addr := <-connection_channel:
                head_socket, err := net.DialUDP("udp", out_addr, addr);
                in_channel <- message.New(CONNECTION)
            }
        }
    }
    return send_channel, broadcast_channel, connect_channel;
}

/*func send(obj Message, socket *net.UDPConn) (error) {
    obj.Signatures = append(obj.Signatures, in_addr.String());
    payload := json.Marshal(obj);
    _, err := socket.Write(buffer);
    return err;
}

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
var head_socket, broadcast_socket *net.UDPConn;

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

    broadcast_socket = net.DialUDP("udp", out_addr, broadcast_addr);

    tail_timeout_channel := nil;
    cycle_timeout_channel := nil;

    push_channel, pop_channel := buffer();
    rcv_channel := listener(pop_channel);
    snd_channel, broadcast_channel, connection_channel := speaker(push_channel);
 
    message_channel := make(chan Message);
    go func() {
        for {
            if head_addr == nil {
                head_request := message.New(HEAD_REQUEST, )
                broadcast_channel <-head_request;
                select {
                case <- time.After(5 * time.Second):
                    continue;
                case rcv := <-msgCh:
                    if rcv.Code == TAIL_REQUEST {
                        connection_channel <-rcv.Body;
                    }
                }
            } else {
                select {
                case <- Time.After(4):
                    send(KEEP_ALIVE, "");
                    timeOutCh = Time.After(8);
                case msg :=  <-rcv_channel:
                    switch msg.Code {
                    case KEEP_ALIVE:
                        break;
                    case CONNECTION:
                        if head_socket == nil || head_socket == rcv.Body.connectee {
                            connection_channel <-rcv.Body.connector);
                        } else {
                            speaker <- msg;
                        }
                    }
                    timeOutCh = nil;
                case <- timeoutCh:
                    //SLEEP SOME TIME
                    send(TAIL_DEAD, "");
                    head_socket = nil;
                }
            }
        } 
    }
    return message_channel;
}