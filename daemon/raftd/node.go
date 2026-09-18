package raftd

import (
	"strconv"
	"strings"

	"codeberg.org/donkeywon/golib/errs"
)

type Node struct {
	ID   string `json:"id"   yaml:"id"   validate:"required"`
	Host string `json:"host" yaml:"host" validate:"required"`
	Port uint16 `json:"port" yaml:"port" validate:"required"`
}

func (n *Node) UnmarshalFlag(value string) error {
	parts := strings.Split(value, ":")
	if len(parts) != 3 {
		return errs.Errorf("invalid node %q: want id:host:port (host must not contain ':')", value)
	}

	port, err := strconv.ParseUint(strings.TrimSpace(parts[2]), 10, 16)
	if err != nil {
		return errs.Wrapf(err, "invalid node %q: invalid port %q", value, parts[2])
	}
	if port == 0 {
		return errs.Errorf("invalid node %q: port must not be 0", value)
	}

	id := strings.TrimSpace(parts[0])
	host := strings.TrimSpace(parts[1])
	if id == "" || host == "" {
		return errs.Errorf("invalid node %q: empty id or host", value)
	}

	n.ID = id
	n.Host = host
	n.Port = uint16(port)
	return nil
}

func (n Node) MarshalFlag() (string, error) {
	return n.ID + ":" + n.Addr(), nil
}

func (n Node) Addr() string {
	return n.Host + ":" + strconv.FormatUint(uint64(n.Port), 10)
}
