package ftp

import (
	"bufio"
	"errors"
	"net"
	"net/textproto"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/donkeywon/golib/errs"
)

const (
	defaultTimeout       = 5
	keepaliveInterval    = 10
	pasvRespDataSplitLen = 6
)

var location, _ = time.LoadLocation("Asia/Shanghai")

type Cfg struct {
	Addr    string `json:"addr"    validate:"required" yaml:"addr"`
	User    string `json:"user"    validate:"required" yaml:"user"`
	Pwd     string `json:"pwd"     yaml:"pwd"`
	Timeout int    `json:"timeout" validate:"gte=1"    yaml:"timeout"`
	Retry   int    `json:"retry"   validate:"gte=1"    yaml:"retry"`
}

func NewCfg() *Cfg {
	return &Cfg{
		Timeout: defaultTimeout,
	}
}

type Client struct {
	*Cfg

	netConn net.Conn
	conn    *textproto.Conn

	closed chan struct{}
}

func NewClient() *Client {
	return &Client{
		Cfg:    NewCfg(),
		closed: make(chan struct{}),
	}
}

func (c *Client) Init() error {
	var err error

	err = retry.Do(
		func() error {
			c.netConn, err = net.DialTimeout("tcp", c.Addr, time.Second*time.Duration(c.Timeout))
			if err != nil {
				return errs.Wrap(err, "ftp dial fail")
			}
			return nil
		},
		retry.Attempts(uint(c.Retry)),
	)
	if err != nil {
		return errs.Wrap(err, "connect to ftp server fail with max retry")
	}

	c.netConn, err = net.DialTimeout("tcp", c.Addr, time.Second*time.Duration(c.Timeout))
	if err != nil {
		return errs.Wrap(err, "ftp dial fail")
	}

	c.conn = textproto.NewConn(c.netConn)

	err = retry.Do(
		func() error {
			_, _, err = c.conn.ReadResponse(StatusReady)
			if err != nil {
				return errors.Join(err, c.Quit())
			}

			err = c.login()
			if err != nil {
				return errs.Wrap(err, "login fail")
			}
			return nil
		},
		retry.Attempts(uint(c.Retry)),
	)

	go c.keepalive()

	return nil
}

func (c *Client) keepalive() {
	t := time.NewTicker(time.Second * keepaliveInterval)
	defer t.Stop()
	for {
		select {
		case <-c.closed:
			return
		case <-t.C:
			_, _, _ = c.cmd(StatusCommandOK, "NOOP")
		}
	}
}

func (c *Client) cmdDataConn(format string, args ...any) (net.Conn, error) {
	conn, err := c.openDataConn()
	if err != nil {
		return nil, errs.Wrap(err, "open data conn fail")
	}

	_, err = c.conn.Cmd(format, args...)
	if err != nil {
		err = errors.Join(err, conn.Close())
		return nil, errs.Wrap(err, "cmd conn fail")
	}

	code, msg, err := c.conn.ReadResponse(-1)
	if err != nil {
		err = errors.Join(err, conn.Close())
		return nil, errs.Wrap(err, "read cmd conn response fail")
	}
	if code != StatusAlreadyOpen && code != StatusAboutToSend {
		conn.Close()
		return nil, &textproto.Error{Code: code, Msg: msg}
	}

	return conn, nil
}

func (c *Client) cmd(expected int, format string, args ...any) (int, string, error) {
	_, err := c.conn.Cmd(format, args...)
	if err != nil {
		return 0, "", err
	}

	return c.conn.ReadResponse(expected)
}

func (c *Client) RawCmd(format string, args ...any) (int, string, error) {
	return c.cmd(-1, format, args...)
}

func (c *Client) openDataConn() (net.Conn, error) {
	host, port, err := c.pasv()
	if err != nil {
		return nil, errs.Wrap(err, "pasv fail")
	}

	addr := net.JoinHostPort(host, strconv.Itoa(port))
	return net.DialTimeout("tcp", addr, time.Second*time.Duration(c.Timeout))
}

func (c *Client) pasv() (string, int, error) {
	_, line, err := c.cmd(StatusPassiveMode, "PASV")
	if err != nil {
		return "", 0, err
	}

	start := strings.Index(line, "(")
	end := strings.LastIndex(line, ")")
	if start == -1 || end == -1 {
		return "", 0, errs.New("invalid PASV response format")
	}

	// We have to split the response string
	pasvData := strings.Split(line[start+1:end], ",")

	if len(pasvData) < pasvRespDataSplitLen {
		return "", 0, errs.New("invalid PASV response format")
	}

	// Let's compute the port number
	portPart1, err := strconv.Atoi(pasvData[4])
	if err != nil {
		return "", 0, err
	}

	portPart2, err := strconv.Atoi(pasvData[5])
	if err != nil {
		return "", 0, err
	}

	// Recompose port
	port := portPart1*256 + portPart2

	// Make the IP address to connect to
	host := strings.Join(pasvData[0:4], ".")
	return host, port, nil
}

func (c *Client) Delete(path string) error {
	_, _, err := c.cmd(StatusRequestedFileActionOK, "DELE %s", path)
	if err != nil {
		return errs.Wrap(err, "ftp cmd DELE fail")
	}
	return nil
}

func (c *Client) login() error {
	code, msg, err := c.cmd(-1, "USER %s", c.User)
	if err != nil {
		return errs.Wrap(err, "ftp cmd USER fail")
	}

	switch code {
	case StatusLoggedIn:
	case StatusUserOK:
		_, _, err = c.cmd(StatusLoggedIn, "PASS %s", c.Pwd)
		if err != nil {
			return errs.Wrap(err, "ftp cmd PASS fail")
		}
	default:
		return errs.Errorf("ftp cmd USER response code unknown: %d, msg: %s", code, msg)
	}

	return nil
}

func (c *Client) DirExists(path string) (bool, error) {
	dir := filepath.Base(path)
	parentPath := filepath.Dir(path)
	entrys, err := c.List(parentPath)
	if err != nil {
		return false, errs.Wrap(err, "LIST fail")
	}
	for _, entry := range entrys {
		if entry.Name == dir && entry.Type == EntryTypeFolder {
			return true, nil
		}
	}
	return false, nil
}

func (c *Client) Mkdir(name string) error {
	_, _, err := c.cmd(StatusPathCreated, "MKD %s", name)
	if err != nil {
		return errs.Wrap(err, "ftp cmd MKD fail")
	}
	return nil
}

func (c *Client) MkdirRecur(path string) error {
	paths := strings.Split(strings.Trim(path, "/\\ "), string(os.PathSeparator))
	if len(paths) == 0 {
		return nil
	}

	for i := 0; i < len(paths); i++ {
		if path == "." || path == ".." {
			continue
		}

		entrys, err := c.List("")
		if err != nil {
			return errs.Wrap(err, "LIST fail")
		}

		pathExists, isDir := c.pathExists(paths[i], entrys)
		if pathExists && !isDir {
			return errs.Errorf("path exists but is not directory: %s", paths[i])
		}
		if !pathExists {
			err = c.Mkdir(paths[i])
			if err != nil {
				return errs.Wrap(err, "MKD fail")
			}
		}

		err = c.ChangeDir(paths[i])
		if err != nil {
			return errs.Wrap(err, "CWD fail")
		}
	}

	return nil
}

func (c *Client) pathExists(path string, entrys []*Entry) (bool, bool) {
	var (
		pathExists bool
		isDir      bool
	)

	for _, entry := range entrys {
		if entry.Name != path {
			continue
		}

		pathExists = true

		if entry.Type == EntryTypeFolder {
			isDir = true
		}

		break
	}
	return pathExists, isDir
}

func (c *Client) checkDataShut() error {
	_, _, err := c.conn.ReadResponse(StatusClosingDataConnection)
	if err != nil {
		return errs.Wrap(err, "read response StatusClosingDataConnection fail")
	}
	return nil
}

func (c *Client) ChangeDir(path string) error {
	_, _, err := c.cmd(StatusRequestedFileActionOK, "CWD %s", path)
	if err != nil {
		return errs.Wrap(err, "ftp cmd CWD fail")
	}
	return nil
}

func (c *Client) CurrentDir() (string, error) {
	_, msg, err := c.cmd(StatusPathCreated, "PWD")
	if err != nil {
		return "", err
	}

	start := strings.Index(msg, "\"")
	end := strings.LastIndex(msg, "\"")

	if start == -1 || end == -1 {
		return "", errs.New("unsuported PWD response format")
	}

	return msg[start+1 : end], nil
}

func (c *Client) NameList(path string) ([]string, error) {
	var (
		entries []string
		err     error
	)

	space := " "
	if path == "" {
		space = ""
	}
	conn, err := c.cmdDataConn("NLST%s%s", space, path)
	if err != nil {
		return nil, err
	}

	r := &Response{conn: conn, c: c}

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		entries = append(entries, scanner.Text())
	}

	return entries, errors.Join(scanner.Err(), r.Close())
}

func (c *Client) List(path string) ([]*Entry, error) {
	var (
		entries []*Entry
		err     error
	)

	cmd := "LIST"
	parser := parseListLine

	space := " "
	if path == "" {
		space = ""
	}
	conn, err := c.cmdDataConn("%s%s%s", cmd, space, path)
	if err != nil {
		return nil, err
	}

	r := &Response{conn: conn, c: c}

	scanner := bufio.NewScanner(r)
	now := time.Now()
	for scanner.Scan() {
		entry, errParse := parser(scanner.Text(), now, location)
		if errParse == nil {
			entries = append(entries, entry)
		}
	}

	err = errors.Join(scanner.Err(), r.Close())

	return entries, err
}

func (c *Client) TransType(typ string) error {
	_, _, err := c.cmd(StatusCommandOK, "TYPE "+typ)
	if err != nil {
		return errs.Wrap(err, "ftp cmd TYPE fail")
	}
	return nil
}

func (c *Client) Close() error {
	return c.Quit()
}

func (c *Client) Quit() error {
	var err error
	_, err = c.conn.Cmd("QUIT")
	if err != nil {
		err = errs.Wrap(err, "ftp cmd QUIT fail")
	}
	closeErr := c.conn.Close()
	if closeErr != nil {
		err = errors.Join(err, errs.Wrap(closeErr, "close ftp conn fail"))
	}
	return err
}
