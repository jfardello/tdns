package plugin

import (
	"context"
	"github.com/jfardello/tdns/config"
	"github.com/miekg/dns"
)

type Ptype int

const (
	PreRouting Ptype = iota
	Resolving
	PostRouting
)

type Plugin interface {
	Run(context.Context, *dns.Msg) (*dns.RR, bool, error)
	Info() (string, Ptype)
	Config(config.Config) error
	Init() error
}
