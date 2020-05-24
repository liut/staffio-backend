package ldap

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEntryType(t *testing.T) {
	base := "dc=example,dc=org"
	name := "nick"
	assert.Equal(t, "uid="+name+",ou=people,"+base, etPeople.DN(name, base))
	assert.Equal(t, "cn="+name+",ou=groups,"+base, etGroup.DN(name, base))
	assert.Equal(t, "CN="+name+",CN=Users,"+base, etADuser.DN(name, base))
	assert.Equal(t, "CN="+name+",CN=Builtin,"+base, etADgroup.DN(name, base))
}

func TestSplitDC(t *testing.T) {
	base := "dc=example,dc=org"
	dc1 := splitDC(base)

	if dc1 != "example" {
		t.Errorf("mismatch %q and %q", dc1, "example")
	}
}

func TestConfig(t *testing.T) {
	c := zeroConfig
	ec := NewConfig()
	c.CopyFrom(*ec)

	assert.NotEmpty(t, c.Addr)
	assert.NotEmpty(t, c.Base)

	t.Logf("test base %s", c.Base)
}
