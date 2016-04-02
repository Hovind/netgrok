package buffer

func Init() (chan<- Message, chan<- Message) {
    push_channel := make(chan Message);
    pop_channel := make(chan Message);
    
    buffer = []Message {}
    
    go func() {
        for {
            select {
            case asd1 := <-push_channel:
                buffer = append
            case msg := <-push_channel:
                for i in range buffer {
                    if buffer[i].Code == msg.Code && bytes.Equal(buffer[i].Body, msg.Body) {
                        buffer = append(buffer[:i], buffer[i+1:]...)
                        break;
                    }
                }
            }
        }
    }
    return push_channel, pop_channel;
}