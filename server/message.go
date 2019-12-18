package main

import (
	"bytes"
	"errors"
)

type message struct {
	bytes  []byte
	author *user
}

func (m message) print() ([]byte, error) {
	buffer := bytes.NewBufferString(m.author.name + ": ")
	nWrite, err := buffer.Write(m.bytes)
	if err != nil {
		return []byte{}, err
	}
	if nWrite != len(m.bytes) {
		return []byte{}, errors.New("Error creating the message")
	}
	return buffer.Bytes(), nil
}
