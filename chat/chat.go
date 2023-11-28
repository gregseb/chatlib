package chat

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"syscall"
)

const (
	RoleAdmin = "admin"
	RoleStaff = "staff"
	RoleUser  = "user"
)

type Message struct {
	Text     string
	Command  string
	Sender   string
	Receiver string
}

type API interface {
	SendMessage(c context.Context, msg *Message) error
	ReceiveMessage(c context.Context) (*Message, error)
	Start(c context.Context) error
	Stop(c context.Context) error
}

type ActionFunc func(c context.Context, re *regexp.Regexp, msg *Message) error

type Action struct {
	Command string
	re      *regexp.Regexp
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

func RegisterAction(command, pattern, example, help string, fn ActionFunc, roles ...string) Option {
	return func(h *Handler) error {
		if h.actions == nil {
			h.actions = make([]*Action, 0)
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return err
		}
		h.actions = append(h.actions, &Action{command, re, example, help, roles, fn})
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
	go h.run(c, cancel)
	go h.listen(c, cancel)
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

func (h *Handler) run(c context.Context, cancel context.CancelFunc) error {
	for {
		select {
		case <-c.Done():
			return c.Err()
		case msg := <-h.msg:
			if msg == nil {
				continue
			}
			for _, action := range h.actions {
				if action.Command == msg.Command && action.re.MatchString(msg.Text) {
					if err := action.fn(c, action.re, msg); err != nil {
						cancel()
						return err
					}
				}
			}
		}
	}
}

func (h *Handler) listen(c context.Context, cancel context.CancelFunc) error {
	for {
		msg, err := (h.api).ReceiveMessage(c)
		if err != nil {
			//cancel()
			//return err
			fmt.Printf("+error: %s\n", err)
		}
		h.msg <- msg
	}
}
