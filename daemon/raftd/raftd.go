package raftd

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/donkeywon/golib/boot"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util/v"
	"github.com/DeRuina/timberjack"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
	"github.com/rs/zerolog"
)

var (
	ErrDuplicateNode = errors.New("duplicate node id or addr")
	ErrNodeNotExists = errors.New("node not exists")
	ErrNotReady      = errors.New("not ready")
)

const DaemonTypeRaftd boot.DaemonType = "raftd"

type Raftd struct {
	cfg Cfg

	l *zerolog.Logger

	r    *raft.Raft
	ob   *raft.Observer
	obCh chan raft.Observation

	onFailedHeartbeat  []func(context.Context, string, time.Time)
	onResumedHeartbeat []func(context.Context, string)
	onNodeRemoved      []func(context.Context, string, string)
	onNodeAdded        []func(context.Context, string, string)

	fsm raft.FSM
}

func New() boot.Daemon {
	return &Raftd{
		obCh: make(chan raft.Observation, 1024),
	}
}

type initializer interface {
	Init(context.Context) error
}

func (r *Raftd) Init(ctx context.Context) error {
	var err error

	err = v.StructCtx(ctx, &r.cfg)
	if err != nil {
		return errs.Wrap(err, "invalid config")
	}
	var selfFound bool
	for _, node := range r.cfg.Nodes {
		if r.cfg.NodeID == node.ID {
			selfFound = true
			break
		}
	}
	if !selfFound {
		return errs.Errorf("self node not found in config nodes: %v", r.cfg.Nodes)
	}

	r.l = zerolog.Ctx(ctx)

	if r.fsm == nil {
		panic("nil raft fsm")
	}

	if i, ok := r.fsm.(initializer); ok {
		err = i.Init(ctx)
		if err != nil {
			return errs.Wrap(err, "failed to init raft fsm")
		}
	}

	r.r, err = newRaft(&r.cfg, r.fsm, r.buildLogger())
	if err != nil {
		return errs.Wrap(err, "failed to init raft node")
	}

	r.l.Info().Uint64("last", r.r.LastIndex()).Uint64("applied", r.r.AppliedIndex()).Uint64("commit", r.r.CommitIndex()).Msg("raft log index")

	if r.cfg.NodeID == r.cfg.Nodes[0].ID {
		raftBootstrapCfg := raft.Configuration{
			Servers: make([]raft.Server, len(r.cfg.Nodes)),
		}

		for i := range r.cfg.Nodes {
			raftBootstrapCfg.Servers[i] = raft.Server{
				ID:      raft.ServerID(r.cfg.Nodes[i].ID),
				Address: raft.ServerAddress(r.cfg.Nodes[i].Addr()),
			}
		}

		future := r.r.BootstrapCluster(raftBootstrapCfg)
		err = future.Error()
		if err != nil {
			if errors.Is(err, raft.ErrCantBootstrap) {
				r.l.Info().Msg("already bootstrap")
			} else {
				return errs.Wrap(err, "failed to bootstrap raft cluster")
			}
		}
	}

	return r.waitLeader(ctx)
}

func (r *Raftd) waitLeader(ctx context.Context) error {
	t := time.NewTimer(0)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			t.Reset(time.Second)
			_, leaderID := r.LeaderWithID()
			if string(leaderID) != "" {
				r.l.Info().Str("leader_id", string(leaderID)).Msg("wait raft leader done")
				return nil
			}

			r.l.Info().Msg("waiting raft leader")
		}
	}
}

func (r *Raftd) Run(ctx context.Context) error {
	r.ob = raft.NewObserver(r.obCh, false, nil)
	r.r.RegisterObserver(r.ob)
	go r.observe(ctx)

	<-ctx.Done()
	return ctx.Err()
}

func (r *Raftd) SetCfg(cfg Cfg) {
	r.cfg = cfg
}

func (r *Raftd) SetFSM(fsm raft.FSM) {
	r.fsm = fsm
}

func (r *Raftd) State() raft.RaftState {
	return r.r.State()
}

func (r *Raftd) LeaderWithID() (raft.ServerAddress, raft.ServerID) {
	return r.r.LeaderWithID()
}

func (r *Raftd) HasLeader() bool {
	_, leaderID := r.LeaderWithID()
	return string(leaderID) != ""
}

func (r *Raftd) Self() string {
	return r.cfg.NodeID
}

func (r *Raftd) Raft() *raft.Raft {
	return r.r
}

func (r *Raftd) FSM() raft.FSM {
	return r.fsm
}

func (r *Raftd) Nodes() ([]raft.Server, []string, []string, error) {
	if r.r == nil {
		return nil, nil, nil, ErrNotReady
	}
	f := r.r.GetConfiguration()
	err := f.Error()
	if err != nil {
		return nil, nil, nil, errs.Wrap(err, "get raft nodes failed")
	}

	c := f.Configuration()

	ids := make([]string, len(c.Servers))
	addrs := make([]string, len(c.Servers))
	for i, s := range c.Servers {
		ids[i] = string(s.ID)
		addrs[i] = string(s.Address)
	}

	return c.Servers, ids, addrs, nil
}

func (r *Raftd) Node(id string) (raft.Server, error) {
	nodes, _, _, err := r.Nodes()
	if err != nil {
		return raft.Server{}, errs.Wrap(err, "get nodes failed")
	}
	for i := range nodes {
		if nodes[i].ID == raft.ServerID(id) {
			return nodes[i], nil
		}
	}
	return raft.Server{}, ErrNodeNotExists
}

func (r *Raftd) OnFailedHeartbeat(f func(context.Context, string, time.Time)) {
	r.onFailedHeartbeat = append(r.onFailedHeartbeat, f)
}

func (r *Raftd) OnResumedHeartbeat(f func(context.Context, string)) {
	r.onResumedHeartbeat = append(r.onResumedHeartbeat, f)
}

func (r *Raftd) OnNodeRemoved(f func(context.Context, string, string)) {
	r.onNodeRemoved = append(r.onNodeRemoved, f)
}

func (r *Raftd) OnNodeAdded(f func(context.Context, string, string)) {
	r.onNodeAdded = append(r.onNodeAdded, f)
}

func (r *Raftd) AddNode(node Node, timeout time.Duration) error {
	err := v.Struct(&node)
	if err != nil {
		return errs.Wrap(err, "invalid node")
	}

	_, nodeIDs, nodeAddrs, err := r.Nodes()
	if err != nil {
		return errs.Wrap(err, "get nodes failed")
	}
	if slices.Contains(nodeIDs, node.ID) || slices.Contains(nodeAddrs, node.Addr()) {
		return ErrDuplicateNode
	}

	f := r.r.AddVoter(raft.ServerID(node.ID), raft.ServerAddress(node.Addr()), 0, timeout)
	err = f.Error()
	if err != nil {
		return errs.Wrap(err, "add node failed")
	}
	return nil
}

func (r *Raftd) DelNode(node Node, timeout time.Duration) error {
	err := v.Struct(&node)
	if err != nil {
		return errs.Wrap(err, "invalid node")
	}
	_, nodeIDs, nodeAddrs, err := r.Nodes()
	if err != nil {
		return errs.Wrap(err, "get nodes failed")
	}
	if !slices.Contains(nodeIDs, node.ID) || !slices.Contains(nodeAddrs, node.Addr()) {
		return ErrNodeNotExists
	}

	f := r.r.RemoveServer(raft.ServerID(node.ID), 0, timeout)
	err = f.Error()
	if err != nil {
		return errs.Wrap(err, "del node failed")
	}
	return nil
}

func (r *Raftd) TransferLeadershipTo(node Node) error {
	err := v.Struct(&node)
	if err != nil {
		return errs.Wrap(err, "invalid node")
	}
	_, nodeIDs, nodeAddrs, err := r.Nodes()
	if err != nil {
		return errs.Wrap(err, "get nodes failed")
	}
	if !slices.Contains(nodeIDs, node.ID) || !slices.Contains(nodeAddrs, node.Addr()) {
		return ErrNodeNotExists
	}

	f := r.r.LeadershipTransferToServer(raft.ServerID(node.ID), raft.ServerAddress(node.Addr()))
	err = f.Error()
	if err != nil {
		return errs.Wrap(err, "transfer leadership failed")
	}

	return nil
}

func (r *Raftd) ApplyLog(p []byte) (any, error) {
	var err error
	f := r.r.Apply(p, r.cfg.ApplyTimeout)
	err = f.Error()
	if err != nil {
		r.l.Error().Bytes("log", p).Err(err).Msg("failed to apply log to raft")
		return nil, errs.Wrap(err, "failed to apply log to raft")
	}

	resp := f.Response()
	if resp == nil {
		return nil, nil
	}

	if err, ok := resp.(error); ok {
		r.l.Error().Bytes("log", p).Err(err).Msg("failed to apply log to raft")
		return nil, errs.Wrap(err, "failed to apply log to raft")
	}
	return resp, nil
}

func (r *Raftd) Snapshot() error {
	f := r.r.Snapshot()
	err := f.Error()
	if err != nil {
		return errs.Wrap(err, "raft snapshot failed")
	}
	return nil
}

func (r *Raftd) buildLogger() io.Writer {
	if strings.ToLower(r.cfg.LogFilepath) == "stdout" {
		return os.Stdout
	}
	if strings.ToLower(r.cfg.LogFilepath) == "stderr" {
		return os.Stderr
	}
	return &timberjack.Logger{
		Filename:    r.cfg.LogFilepath,
		MaxSize:     r.cfg.LogMaxFileSize,
		MaxAge:      r.cfg.LogMaxAge,
		MaxBackups:  r.cfg.LogMaxBackups,
		LocalTime:   true,
		Compression: r.cfg.LogCompression,
	}
}

func (r *Raftd) observe(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case o := <-r.obCh:
			r.l.Info().Any("observation", o.Data).Msg("raft event observed")

			switch ob := o.Data.(type) {
			case raft.LeaderObservation:
				r.l.Info().Str("leader_id", string(ob.LeaderID)).Str("leader_addr", string(ob.LeaderAddr)).Msg("leader changed")
			case raft.PeerObservation:
				if ob.Removed {
					r.l.Info().Any("node", ob.Peer).Msg("node removed observed")
					for _, f := range r.onNodeRemoved {
						select {
						case <-ctx.Done():
							return
						default:
						}

						func() {
							defer func() {
								p := recover()
								if p != nil {
									r.l.Error().Err(errs.PanicToErr(p)).Msg("panic on call OnNodeRemoved")
								}
							}()

							f(ctx, string(ob.Peer.ID), string(ob.Peer.Address))
						}()
					}
				} else {
					r.l.Info().Any("node", ob.Peer).Msg("node added observed")
					for _, f := range r.onNodeAdded {
						select {
						case <-ctx.Done():
							return
						default:
						}

						func() {
							defer func() {
								p := recover()
								if p != nil {
									r.l.Error().Err(errs.PanicToErr(p)).Msg("panic on call OnNodeAdded")
								}
							}()

							f(ctx, string(ob.Peer.ID), string(ob.Peer.Address))
						}()
					}
				}
			case raft.FailedHeartbeatObservation:
				r.l.Warn().Str("node_id", string(ob.PeerID)).Str("last_contact", ob.LastContact.Format("2006-01-02 15:04:05.000000")).Msg("failed heartbeat observed")
				for _, f := range r.onFailedHeartbeat {
					select {
					case <-ctx.Done():
						return
					default:
					}

					func() {
						defer func() {
							p := recover()
							if p != nil {
								r.l.Error().Err(errs.PanicToErr(p)).Msg("panic on call OnFailedHeartbeat")
							}
						}()

						f(ctx, string(ob.PeerID), ob.LastContact)
					}()
				}

			case raft.ResumedHeartbeatObservation:
				r.l.Info().Str("node_id", string(ob.PeerID)).Msg("resumed heartbeat observed")
				for _, f := range r.onResumedHeartbeat {
					select {
					case <-ctx.Done():
						return
					default:
					}

					func() {
						defer func() {
							p := recover()
							if p != nil {
								r.l.Error().Err(errs.PanicToErr(p)).Msg("panic on call OnResumedHeartbeat")
							}
						}()

						f(ctx, string(ob.PeerID))
					}()
				}
			case raft.RaftState:
				r.l.Info().Str("state", ob.String()).Msg("raft state changed")
			case raft.RequestVoteRequest:
				r.l.Info().Any("req", ob).Msg("vote request observed")
			case raft.RequestPreVoteRequest:
				r.l.Info().Any("req", ob).Msg("pre vote request observed")
			}
		}
	}
}

func newRaft(cfg *Cfg, fsm raft.FSM, logger io.Writer) (r *raft.Raft, err error) {
	rcfg := raft.DefaultConfig()
	rcfg.LocalID = raft.ServerID(cfg.NodeID)
	rcfg.HeartbeatTimeout = cfg.HeartbeatTimeout
	rcfg.ElectionTimeout = cfg.ElectionTimeout
	rcfg.CommitTimeout = cfg.CommitTimeout
	rcfg.SnapshotInterval = cfg.SnapshotInterval
	rcfg.SnapshotThreshold = cfg.SnapshotThreshold
	rcfg.LeaderLeaseTimeout = cfg.LeaderLeaseTimeout
	rcfg.ShutdownOnRemove = false
	rcfg.NoSnapshotRestoreOnStart = cfg.NoSnapshotRestoreOnStart
	rcfg.LogOutput = logger
	rcfg.LogLevel = cfg.LogLevel

	var (
		ls     raft.LogStore
		stable raft.StableStore
		ss     raft.SnapshotStore
		ts     raft.Transport
	)
	defer func() {
		if err == nil || r != nil {
			return
		}
		if c, ok := ts.(io.Closer); ok {
			_ = c.Close()
		}
		if c, ok := ls.(io.Closer); ok {
			_ = c.Close()
		}
		if c, ok := stable.(io.Closer); ok {
			_ = c.Close()
		}
		if c, ok := ss.(io.Closer); ok {
			_ = c.Close()
		}
	}()

	resolvedAdvertiseAddr, err := net.ResolveTCPAddr("tcp", cfg.AdvertiseAddr)
	if err != nil {
		err = errs.Wrap(err, "failed to resolve advertise address")
		return
	}

	ts, err = raft.NewTCPTransport(cfg.ListenAddr, resolvedAdvertiseAddr, cfg.MaxConn, cfg.TransportTimeout, logger)
	if err != nil {
		err = errs.Wrap(err, "failed to create tcp transport")
		return
	}

	ls, err = raftboltdb.NewBoltStore(filepath.Join(cfg.DataDir, "log.db"))
	if err != nil {
		err = errs.Wrap(err, "failed to create raft boltdb log store")
		return
	}

	stable, err = raftboltdb.NewBoltStore(filepath.Join(cfg.DataDir, "stable.db"))
	if err != nil {
		err = errs.Wrap(err, "failed to create raft boltdb stable store")
		return
	}

	ss, err = raft.NewFileSnapshotStore(filepath.Join(cfg.DataDir, "snapshot"), cfg.SnapshotRetainCount, logger)
	if err != nil {
		err = errs.Wrap(err, "failed to create snapshot store")
		return
	}

	r, err = raft.NewRaft(rcfg, fsm, ls, stable, ss, ts)
	if err != nil {
		err = errs.Wrap(err, "failed to create raft node")
		return
	}

	return
}
