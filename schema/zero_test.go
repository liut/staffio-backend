package schema

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPeople(t *testing.T) {
	assert.Panics(t, func() {
		NewPeople()
	})

	p := NewPeople("uid", "cn", "sn", "gn", "nickname")

	assert.Equal(t, "uid", p.GetUID())
	assert.Equal(t, "cn", p.GetCommonName())
	assert.Equal(t, "nickname", p.GetName())
	p.Nickname = ""
	assert.Equal(t, "cn", p.Name())
	p.CommonName = ""
	assert.Equal(t, "gn sn", p.Name())
	assert.Equal(t, "gn sn", p.GetCommonName())

	p.GivenName = ""
	assert.Equal(t, "uid", p.Name())

	assert.Empty(t, p.AvatarURI())
	p.AvatarPath = "a.png"
	assert.Equal(t, "a.png", p.AvatarURI())

	p.AvatarPath = "abc/"
	assert.Equal(t, "abc/0", p.AvatarURI())

	p.AvatarPath = "//abc"
	assert.Equal(t, "//abc", p.AvatarURI())

	p.AvatarPath = "/wwhead/abc"
	assert.Equal(t, URIPrefixQqcn+"/wwhead/abc", p.AvatarURI())

	p.AvatarPath = ""
	jh := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10}
	p.JpegPhoto = jh
	dataURI := URIPrefixData + base64.URLEncoding.EncodeToString(jh)
	assert.Equal(t, dataURI, p.AvatarURI())
}

func TestPeoples(t *testing.T) {
	uid := "uid"
	arr := UIDs{uid}
	assert.True(t, arr.Has(uid))
	assert.False(t, arr.Has("nouid"))

	p := NewPeople(uid)
	pp := Peoples{*p}
	assert.NotNil(t, pp.WithUID(uid))
	assert.Nil(t, pp.WithUID("nouid"))
}

func TestGroup(t *testing.T) {
	g := &Group{
		Name:        "g1",
		Description: "gt1",
		Members:     []string{"uid", "uid2"},
	}

	assert.True(t, g.Has("uid"))
}
