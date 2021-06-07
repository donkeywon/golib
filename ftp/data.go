// Copyright (c) 2011-2013, Julien Laffaye <jlaffaye@FreeBSD.org>

// Permission to use, copy, modify, and/or distribute this software for any
// purpose with or without fee is hereby granted, provided that the above
// copyright notice and this permission notice appear in all copies.

// THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
// WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
// MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
// ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
// WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
// ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
// OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.

// Copied from github.com/jlaffaye/ftp

package ftp

import (
	"errors"
	"net"
	"time"
)

type EntryType int

const (
	EntryTypeFile EntryType = iota
	EntryTypeFolder
	EntryTypeLink
)

type Entry struct {
	Name   string
	Target string // target of symbolic link
	Type   EntryType
	Size   uint64
	Time   time.Time
}

type Response struct {
	conn   net.Conn
	c      *Client
	closed bool
}

func (r *Response) Read(buf []byte) (int, error) {
	return r.conn.Read(buf)
}

func (r *Response) Close() error {
	if r.closed {
		return nil
	}

	r.closed = true
	return errors.Join(r.conn.Close(), r.c.checkDataShut())
}

func (r *Response) SetDeadline(t time.Time) error {
	return r.conn.SetDeadline(t)
}

func (t EntryType) String() string {
	return [...]string{"file", "folder", "link"}[t]
}
