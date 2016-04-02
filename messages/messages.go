package message

type message struct {
    Code int
    Body []byte
    Signatures []string
}

func New(code int, body []byte) *matrix {
    return &message{Code: code, Body: body, []byte{}};
}

const (
    ORDER_PUSH = 100 + iota
    ORDER_POP
    FLOOR_HIT
    DIRECTION_CHANGE
    SYNC_CART
)

const (
    KEEP_ALIVE = 200 + iota
)

const (
    CONNECTION = 300 + iota
    HEAD_REQUEST
    TAIL_REQUEST
)

const (
    TAIL_DEAD = 400 + iota
    CYCLE_BREAK
)