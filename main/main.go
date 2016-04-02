package main

func main() {
    if len(os.Args) < 3 {
        fmt.Println("Too few arguments! I need ports.")
        return;
    }
    network_channel := network.Init(os.Args[1], os.Args[2]);
    for {
    	msg := <-network_channel;

    }
}