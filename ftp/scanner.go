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

// A scanner for fields delimited by one or more whitespace characters.
type scanner struct {
	bytes    []byte
	position int
}

// newScanner creates a new scanner.
func newScanner(str string) *scanner {
	return &scanner{
		bytes: []byte(str),
	}
}

// NextFields returns the next `count` fields.
func (s *scanner) NextFields(count int) []string {
	fields := make([]string, 0, count)
	for i := 0; i < count; i++ {
		if field := s.Next(); field != "" {
			fields = append(fields, field)
		} else {
			break
		}
	}
	return fields
}

// Next returns the next field.
func (s *scanner) Next() string {
	sLen := len(s.bytes)

	// skip trailing whitespace
	for s.position < sLen {
		if s.bytes[s.position] != ' ' {
			break
		}
		s.position++
	}

	start := s.position

	// skip non-whitespace
	for s.position < sLen {
		if s.bytes[s.position] == ' ' {
			s.position++
			return string(s.bytes[start : s.position-1])
		}
		s.position++
	}

	return string(s.bytes[start:s.position])
}

// Remaining returns the remaining string.
func (s *scanner) Remaining() string {
	return string(s.bytes[s.position:len(s.bytes)])
}
