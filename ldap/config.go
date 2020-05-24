package ldap

import (
	"fmt"
	"os"
	"regexp"

	"github.com/go-ldap/ldap/v3"
)

var (
	reUID = regexp.MustCompile("^[a-z][a-z0-9-_]+$")
)

/*

	cfg := NewConfig()
	store, err := NewStore(cfg)
	if err != nil {
		log.Fatalf("new service ERR %s", err)
	}

*/

// Config LDAP config
type Config struct {
	Addr     string `json:"addr"`
	Base     string `json:"base"`
	Bind     string `json:"bind"`
	Passwd   string `json:"-"`
	Domain   string `json:"domain"`
	PageSize int    `json:"-"`
}

var zeroConfig = &Config{}

// NewConfig return default Config from Environment
func NewConfig() *Config {
	return &Config{
		Addr:     envOr("LDAP_HOSTS", envOr("STAFFIO_LDAP_HOSTS", "localhost")),
		Base:     envOr("LDAP_BASE", envOr("STAFFIO_LDAP_BASE", "dc=mydomain,dc=net")),
		Domain:   envOr("LDAP_DOMAIN", envOr("STAFFIO_LDAP_DOMAIN", "mydomain.net")),
		Bind:     envOr("LDAP_BIND_DN", envOr("STAFFIO_LDAP_BIND_DN", "")),
		Passwd:   envOr("LDAP_PASSWD", envOr("STAFFIO_LDAP_PASS", "")),
		PageSize: DefaultPageSize,
	}
}

// CopyFrom ...
func (c *Config) CopyFrom(o Config) {
	if o.Addr != "" && o.Addr != c.Addr {
		c.Addr = o.Addr
	}
	if o.Base != "" && o.Base != c.Base {
		c.Base = o.Base
	}
	if o.Domain != "" && o.Domain != c.Domain {
		c.Domain = o.Domain
	}
	if o.Bind != "" && o.Bind != c.Bind {
		c.Bind = o.Bind
	}
	if o.Passwd != "" && o.Passwd != c.Passwd {
		c.Passwd = o.Passwd
	}
}

type entryType struct {
	PK, OC     string
	Filter     string
	Attributes []string
}

// newEentryType ...
func newEentryType(pk, oc string, attrs ...string) (et *entryType) {
	et = &entryType{
		PK:         pk,
		OC:         oc,
		Attributes: attrs,
	}
	if oc == "" {
		oc = "*"
	}
	et.Filter = fmt.Sprintf("(objectclass=%s)", oc)

	return
}

// DN ...
func (et *entryType) DN(name, base string) string {
	switch et {
	case etGroup:
		return makeDN(et.PK, name, etParent.DN("groups", base))
	case etPeople:
		return makeDN(et.PK, name, etParent.DN("people", base))
	case etADgroup:
		return "CN=" + name + ",CN=Builtin," + base
	case etADuser:
		return "CN=" + name + ",CN=Users," + base
	case etBase:
		return base
	}
	// parent
	return makeDN(et.PK, name, base)
}

type attributer interface {
	Attribute(attrType string, attrVals []string)
}

func (et *entryType) objectClasses() []string {
	if et.PK == "dc" {
		return []string{"domain", "top"}
	}
	return []string{et.OC, "top"}
}

func (et *entryType) prepareTo(name string, ar attributer) {
	ar.Attribute("objectClass", et.objectClasses())
	ar.Attribute(et.PK, []string{name})
	if et.PK == "dc" {
		ar.Attribute("l", []string{name})
	}
}

func (et *entryType) oneFilter(value string) string {
	return "(&(" + et.PK + "=" + ldap.EscapeFilter(value) + ")" + et.Filter + ")"
}

// makeDN ...
func makeDN(pk, name, parent string) string {
	return fmt.Sprintf("%s=%s,%s", pk, name, parent)
}

// consts
const (
	TimeLayout = "20060102150405Z"
	DateLayout = "20060102"

	DefaultPageSize = 100
	DefaultPoolSize = 10
)

var (
	etBase   = newEentryType("dc", "", "dc", "o", "instanceType") // dcObject
	etParent = newEentryType("ou", "organizationalUnit", "ou")
	etGroup  = newEentryType("cn", "groupOfNames", "cn", "member")
	etPeople = newEentryType("uid", "inetOrgPerson", "uid",
		"cn", "gn", "sn", "displayName", "mail", "mobile", "description", "metaJSON",
		"createdTime", "modifiedTime", "createTimestamp", "modifyTimestamp", "jpegPhoto",
		"avatarPath", "dateOfBirth", "gender", "employeeNumber", "employeeType", "title")

	etADgroup = newEentryType("cn", "group", "cn", "member", "name", "description", "instanceType")
	etADuser  = newEentryType("cn", "user", "cn", "name", "sAMAccountName", "userPrincipalName",
		"uid", "gn", "sn", "displayName", "mail", "mobile", "description",
		"employeeNumber", "employeeType", "title", "jpegPhoto", "logonCount")

	objectClassPeople = []string{"top", "staffioPerson" /*"uidObject",*/, "inetOrgPerson"}
)

func envOr(key, dft string) string {
	v := os.Getenv(key)
	if v == "" {
		return dft
	}
	return v
}
