package main

import (
    "os"
    "fmt"
    "netgrok/network"
    . "netgrok/obj"
    "bufio"
)

func main() {
    if len(os.Args) < 2 {
        fmt.Println("Too few arguments! I need ports.");
        return;
    }

    to_network_channel, from_network_channel, _, _/*sync_to_order_channel, sync_request_channel*/ := network.Manager(os.Args[1]);
    go func() {
        for {
            msg := <-from_network_channel;
            fmt.Print(string(msg.Body));
        }
    }();
    reader := bufio.NewReader(os.Stdin);
    for {
        msg, err := reader.ReadString('\n');
        if err != nil {
            fmt.Println("Error reading from stdin.");
        }
        to_network_channel <-*NewMessage(ORDER_PUSH, []byte(msg), nil, nil);
    }
}
