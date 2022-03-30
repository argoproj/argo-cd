package grpc

import (
	"github.com/gogo/protobuf/codec"
	"google.golang.org/grpc/encoding"
)

func RegisterGoGoCodec() {
	c := codec.New(0)
	encoding.RegisterCodec(&gogoCodec{c})
}

type gogoCodec struct {
	c codec.Codec
}

func (g *gogoCodec) Marshal(v interface{}) ([]byte, error) {
	return g.c.Marshal(v)
}

func (g *gogoCodec) Unmarshal(data []byte, v interface{}) error {
	return g.c.Unmarshal(data, v)
}

func (g *gogoCodec) Name() string {
	return "proto"
}
