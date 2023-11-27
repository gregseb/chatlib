package chat

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

type Message struct {
	Text     string
	Sender   string
	Receiver string
}

type API interface {
	SendMessage(c context.Context, msg *Message) error
	ReceiveMessage(c context.Context) (*Message, error)
	Start(c context.Context) error
	Stop(c context.Context) error
}

type ActionFunc func(msg *Message) error

type Action struct {
	re      string
	example string
	help    string
	roles   []string
	fn      ActionFunc
}

type Option func(*Handler) error

func WithAPI(api API) Option {
	return func(h *Handler) error {
		h.api = api
		return nil
	}
}

func RegisterAction(re, example, help string, fn ActionFunc, roles ...string) Option {
	return func(h *Handler) error {
		if h.actions == nil {
			h.actions = make([]*Action, 0)
		}
		h.actions = append(h.actions, &Action{re, example, help, roles, fn})
		return nil
	}
}

func OnClose(cb Callback) Option {
	return func(h *Handler) error {
		if h.closeCBs == nil {
			h.closeCBs = make([]Callback, 0)
		}
		h.closeCBs = append(h.closeCBs, cb)
		return nil
	}
}

func CombineOptions(opts ...Option) Option {
	return func(h *Handler) error {
		return h.ApplyOptions(opts...)
	}
}

func (h *Handler) ApplyOptions(opts ...Option) error {
	for _, opt := range opts {
		if err := opt(h); err != nil {
			return err
		}
	}
	return nil
}

type Callback func() error

type Handler struct {
	api      API
	msg      chan *Message
	actions  []*Action
	closeCBs []Callback
}

func New(opts ...Option) (*Handler, error) {
	h := &Handler{
		msg: make(chan *Message),
	}
	if err := h.ApplyOptions(opts...); err != nil {
		return nil, err
	}
	return h, nil
}

func (h *Handler) Start(ctx context.Context) error {
	c, cancel := context.WithCancel(ctx)
	if err := h.api.Start(c); err != nil {
		cancel()
		return err
	}
	go h.Run(c)
	go h.Listen(c)
	// Listen for SigInt
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT)
	go func() {
		<-sigs
		cancel()
	}()
	<-ctx.Done()
	h.api.Stop(c)
	return nil
}

func (h *Handler) Run(c context.Context) error {
	for {
		select {
		case <-c.Done():
			return c.Err()
		case msg := <-h.msg:
			for _, action := range h.actions {
				if action.re == msg.Text {
					if err := action.fn(msg); err != nil {
						return err
					}
				}
			}
		}
	}
}

func (h *Handler) Listen(c context.Context) error {
	for {
		msg, err := (h.api).ReceiveMessage(c)
		if err != nil {
			return err
		}
		h.msg <- msg
	}
}
