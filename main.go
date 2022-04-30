package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/netip"
	"os"
	"os/signal"

	"go.uber.org/zap"
	yaml "gopkg.in/yaml.v2"
)

var debug bool
var listenAddr string
var configFile string

var logger *zap.Logger

// Config represents a repeater configuration.
type Config struct {
	ListenPorts []int             `yaml:"listenPorts"`
	Targets     []*netip.AddrPort `yaml:"targets"`
}

func init() {
	flag.BoolVar(&debug, "debug", false, "debug logging")
	flag.StringVar(&configFile, "c", "config.yaml", "configuration map")
}

func main() {
	var err error

	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if debug {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}

	if err != nil {
		log.Fatalln("failed to create logger:", err)
	}

	logger.Debug("Debug mode enabled")

	cfg := new(Config)

	f, err := os.Open(configFile)
	if err != nil {
		logger.Sugar().Fatalf("failed to open config file %s: %s", configFile, err.Error())
	}

	if err := yaml.NewDecoder(f).Decode(cfg); err != nil {
		logger.Sugar().Fatalf("failed to parse config file %s: %s", configFile, err.Error())
	}

	d := NewDistributor(ctx, cfg.Targets)

	for _, p := range cfg.ListenPorts {
		_, err := NewReceiver(ctx, fmt.Sprintf(":%d", p), d)
		if err != nil {
			logger.Sugar().Fatal("failed to start receiver:", err)
		}
	}

	<-ctx.Done()
}

// Distributor provides an exchange in which anything sent to it is sent ot any targets registered to it.
type Distributor struct {
	senders []*Sender
	ch      chan []byte
}

func NewDistributor(ctx context.Context, targets []*netip.AddrPort) *Distributor {
	var senders []*Sender

	for _, t := range targets {
		conn, err := net.DialUDP("udp", nil, net.UDPAddrFromAddrPort(*t))
		if err != nil {
			logger.Fatal("failed to dial target",
				zap.String("target", t.String()),
				zap.Error(err),
			)
		}

		senders = append(senders, NewSender(ctx, conn))
	}

	d := &Distributor{
		senders: senders,
		ch:      make(chan []byte, 100),
	}

	go d.run(ctx)

	return d
}

func (d *Distributor) run(ctx context.Context) {
	for {
		select {
		case m := <-d.ch:
			logger.Debug("received message", zap.String("msg", string(m)))
			for _, s := range d.senders {
				s.Send(m)
			}
		case <-ctx.Done():
			return
		}
	}
}

// Send transmits a message to the Distributor for distribution.
func (d *Distributor) Send(m []byte) {
	select {
	case d.ch <- m:
	default:
	}
}

// Sender ships a message received to its designated target, buffering as necessary, dropping if need be.  It will never block.
type Sender struct {
	ch chan []byte
	t  *net.UDPConn
}

func NewSender(ctx context.Context, target *net.UDPConn) *Sender {
	s := &Sender{
		ch: make(chan []byte, 100),
		t:  target,
	}

	go s.run(ctx)

	return s
}

func (s *Sender) run(ctx context.Context) {
	logger.Info("starting new Sender", zap.String("target", s.t.RemoteAddr().String()))

	for {
		select {
		case m := <-s.ch:
			logger.Debug("sending message",
				zap.String("msg", string(m)),
				zap.String("target", s.t.RemoteAddr().String()),
			)

			n, err := s.t.Write(m)
			if err != nil {
				logger.Error("failed to write message",
					zap.String("msg", string(m)),
					zap.String("target", s.t.RemoteAddr().String()),
					zap.Error(err),
				)
			}

			if n != len(m) {
				logger.Error("unexpected write length",
					zap.String("msg", string(m)),
					zap.String("target", s.t.RemoteAddr().String()),
					zap.Int("expected", len(m)),
					zap.Int("sent", n),
				)
			}

		case <-ctx.Done():
			return
		}
	}
}

// Send sends a message to the target of the Sender.
func (s *Sender) Send(m []byte) {
	select {
	case s.ch <- m:
	default:
	}
}

// Receiver listens on a port for OSC messages, sending any received messages to the Distrubtor.
type Receiver struct {
	d *Distributor
	s *net.UDPConn
}

// NewReceiver creates a new OSC receiver
func NewReceiver(ctx context.Context, addr string, d *Distributor) (*Receiver, error) {
	uaddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse listen address %q: %w", addr, err)
	}

	conn, err := net.ListenUDP("udp", uaddr)
	if err != nil {
		return nil, fmt.Errorf("failed to dial address %s: %w", addr, err)
	}

	go func() {
		logger.Info("starting receiver", zap.String("socket", addr))

		defer func() {
			if err := conn.Close(); err != nil {
				logger.Error("failed to close listener on context closure",
					zap.Error(err),
				)
			}
		}()

		for {
			b := make([]byte, 256)


			n, err := conn.Read(b)
			if err != nil {
				logger.Fatal("failed to read from socket",
					zap.String("address", addr),
					zap.Error(err),
				)
				return
			}
			if n == 0 {
				logger.Debug("empty")
				continue 
			}

			logger.Debug("received data", zap.ByteString("data", b[:n]))

			d.Send(b[:n])
		}
	}()

	return &Receiver{
		d: d,
		s: conn,
	}, nil
}
