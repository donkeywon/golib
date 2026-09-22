package raftd

import (
	"time"
)

const (
	DefaultHeartbeatTimeout    = 1 * time.Second
	DefaultElectionTimeout     = 1 * time.Second
	DefaultCommitTimeout       = 50 * time.Millisecond
	DefaultSnapshotInterval    = 120 * time.Second
	DefaultLeaderLeaseTimeout  = 500 * time.Millisecond
	DefaultTransportTimeout    = 10 * time.Second
	DefaultApplyTimeout        = 10 * time.Second
	DefaultSnapshotRetainCount = 10
	DefaultMaxConn             = 3

	DefaultLogMaxFileSize = 100
	DefaultLogMaxBackups  = 30
	DefaultLogMaxAge      = 30
	DefaultLogCompression = "zstd"
	DefaultLogLevel       = "debug"

	DefaultSnapshotThreshold        = 8192
	DefaultNoSnapshotRestoreOnStart = false

	DefaultDataDir     = "raft"
	DefaultLogFilepath = "raft.log"
)

type Cfg struct {
	DataDir       string `env:"DATA_DIR"       long:"data-dir"       yaml:"dataDir"       description:"raft data dir"                          validate:"required"`
	ListenAddr    string `env:"LISTEN_ADDR"    long:"listen-addr"    yaml:"listenAddr"    description:"raft listen address"                    validate:"required"`
	AdvertiseAddr string `env:"ADVERTISE_ADDR" long:"advertise-addr" yaml:"advertiseAddr" description:"raft advertise address"                 validate:"required"`
	NodeID        string `env:"NODE_ID"        long:"node-id"        yaml:"nodeID"        description:"raft node id"                           validate:"required"`
	Nodes         []Node `env:"NODES"          long:"nodes"          yaml:"nodes"         description:"raft nodes list, format = id:host:port" validate:"required,min=1,dive" env-delim:","`

	ApplyTimeout        time.Duration `env:"APPLY_TIMEOUT"         long:"apply-timeout"         yaml:"applyTimeout"        description:"limit the amount of time wait for the command to be started"`
	HeartbeatTimeout    time.Duration `env:"HEARTBEAT_TIMEOUT"     long:"heartbeat-timeout"     yaml:"heartbeatTimeout"    description:"heartbeat timeout"`
	ElectionTimeout     time.Duration `env:"ELECTION_TIMEOUT"      long:"election-timeout"      yaml:"electionTimeout"     description:"election timeout"`
	CommitTimeout       time.Duration `env:"COMMIT_TIMEOUT"        long:"commit-timeout"        yaml:"commitTimeout"       description:"commit timeout"`
	SnapshotInterval    time.Duration `env:"SNAPSHOT_INTERVAL"     long:"snapshot-interval"     yaml:"snapshotInterval"    description:"snapshot interval"`
	SnapshotThreshold   uint64        `env:"SNAPSHOT_THRESHOLD"    long:"snapshot-threshold"    yaml:"snapshotThreshold"   description:"snapshot threshold"`
	LeaderLeaseTimeout  time.Duration `env:"LEADER_LEASE_TIMEOUT"  long:"leader-lease-timeout"  yaml:"leaderLeaseTimeout"  description:"leader lease timeout"`
	TransportTimeout    time.Duration `env:"TRANSPORT_TIMEOUT"     long:"transport-timeout"     yaml:"transportTimeout"    description:"transport timeout"`
	SnapshotRetainCount int           `env:"SNAPSHOT_RETAIN_COUNT" long:"snapshot-retain-count" yaml:"snapshotRetainCount" description:"snapshot retain count"`
	MaxConn             int           `env:"MAX_CONN"              long:"max-conn"              yaml:"maxConn"             description:"max connections"`

	LogFilepath    string `env:"LOG_FILEPATH"      long:"log-filepath"      yaml:"logFilepath"    description:"raft log file path"`
	LogMaxFileSize int    `env:"LOG_MAX_FILE_SIZE" long:"log-max-file-size" yaml:"logMaxFileSize" description:"maximum size in megabytes of the log file before it gets rotated"`
	LogMaxBackups  int    `env:"LOG_MAX_BACKUPS"   long:"log-max-backups"   yaml:"logMaxBackups"  description:"maximum number of old log files to retain"`
	LogMaxAge      int    `env:"LOG_MAX_AGE"       long:"log-max-age"       yaml:"logMaxAge"      description:"maximum number of days to retain old log files based on the timestamp encoded in their filename"`
	LogLevel       string `env:"LOG_LEVEL"         long:"log-level"         yaml:"logLevel"       description:"minimum enabled logging level"`
	LogCompression string `env:"LOG_COMPRESSION"   long:"log-compression"   yaml:"logCompression" description:"gzip/zstd/none"`

	NoSnapshotRestoreOnStart bool `env:"NO_SNAPSHOT_RESTORE_ON_START" long:"no-snapshot-restore-on-start" yaml:"noSnapshotRestoreOnStart" description:"no snapshot restore on start"`
}

func NewCfg() Cfg {
	return Cfg{
		DataDir:                  DefaultDataDir,
		ApplyTimeout:             DefaultApplyTimeout,
		HeartbeatTimeout:         DefaultHeartbeatTimeout,
		ElectionTimeout:          DefaultElectionTimeout,
		CommitTimeout:            DefaultCommitTimeout,
		SnapshotInterval:         DefaultSnapshotInterval,
		LeaderLeaseTimeout:       DefaultLeaderLeaseTimeout,
		TransportTimeout:         DefaultTransportTimeout,
		SnapshotRetainCount:      DefaultSnapshotRetainCount,
		SnapshotThreshold:        DefaultSnapshotThreshold,
		MaxConn:                  DefaultMaxConn,
		LogFilepath:              DefaultLogFilepath,
		LogMaxFileSize:           DefaultLogMaxFileSize,
		LogMaxBackups:            DefaultLogMaxBackups,
		LogMaxAge:                DefaultLogMaxAge,
		LogLevel:                 DefaultLogLevel,
		LogCompression:           DefaultLogCompression,
		NoSnapshotRestoreOnStart: DefaultNoSnapshotRestoreOnStart,
	}
}
