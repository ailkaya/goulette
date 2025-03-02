package core

import (
	"bufio"
	"encoding/json"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	LogPath        = "log"
	MaxLogFileSize = 1024 * 1024 * 512
)

var writeR *writer
var once sync.Once

type writer struct {
	sync.Mutex
	flushWhenWrite bool
	size           int
	fd             *os.File
	buf            *bufio.Writer
	latestIDs      map[string]uint64
}

func newWriter(flushWhenWrite bool) (*writer, error) {
	var err error
	once.Do(func() {
		writeR = &writer{
			flushWhenWrite: flushWhenWrite,
			size:           0,
			latestIDs:      make(map[string]uint64),
		}
		if err = os.MkdirAll(LogPath, 0777); err != nil {
			return
		}
		if err = writeR.newMsgFile(); err != nil {
			return
		}
		writeR.buf = bufio.NewWriter(writeR.fd)
	})
	if err != nil {
		return nil, err
	}
	return writeR, nil
}

type line struct {
	topic string
	data  []byte
}

func (w *writer) write(content *line) (uint64, error) {
	w.Lock()
	defer w.Unlock()

	id := w.latestIDs[content.topic]
	builder := strings.Builder{}
	builder.WriteString(content.topic)
	builder.WriteString(" ")
	builder.WriteString(strconv.FormatUint(id, 10))
	builder.WriteString(" ")
	builder.WriteString(string(content.data))
	builder.WriteString("\n")
	data := []byte(builder.String())

	_, err := w.buf.Write(data)
	if err != nil {
		return 0, err
	}
	w.latestIDs[content.topic]++
	w.size += len(data)
	if w.size > MaxLogFileSize {
		if err = w.newIndexFile(); err != nil {
			return 0, err
		}
		if err = w.newMsgFile(); err != nil {
			return 0, err
		}
		if err = w.buf.Flush(); err != nil {
			return 0, err
		}
		if err = w.fd.Close(); err != nil {
			return 0, err
		}
		w.buf.Reset(w.fd)
		w.size = 0
		return 0, nil
	}
	if w.flushWhenWrite {
		if err = w.buf.Flush(); err != nil {
			return 0, err
		}
	}
	return id, nil
}

// 创建一个新的记录文件
func (w *writer) newMsgFile() error {
	filename := LogPath + "/" + strconv.FormatInt(time.Now().UnixNano(), 16) + ".msg"
	fd, err := os.Create(filename)
	if err != nil {
		return err
	}
	w.fd = fd
	return nil
}

// 创建一个新的索引文件
func (w *writer) newIndexFile() error {
	filename := LogPath + "/" + w.fd.Name() + ".idx"
	fd, err := os.Create(filename)
	if err != nil {
		return err
	}
	buf := bufio.NewWriter(fd)
	for key, value := range w.latestIDs {
		builder := &strings.Builder{}
		builder.WriteString(key)
		builder.WriteString(" ")
		builder.WriteString(strconv.FormatUint(value, 10))
		builder.WriteString("\n")
		if _, err = buf.WriteString(builder.String()); err != nil {
			return err
		}
	}
	if err = buf.Flush(); err != nil {
		return err
	}
	return fd.Close()
}

func serialize(obj *line) []byte {
	serialized, err := json.Marshal(obj)
	if err != nil {
		log.Fatal(err)
	}
	return serialized
}
