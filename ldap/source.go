package ldap

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-ldap/ldap/v3"
	godebug "github.com/wealthworks/go-debug"

	"github.com/liut/staffio-backend/ldap/pool"
	"github.com/liut/staffio-backend/schema"
)

// PoolStats ...
type PoolStats = pool.Stats

// Group ...
type Group = schema.Group

// People ...
type People = schema.People

// Peoples ...
type Peoples = schema.Peoples

// Spec ...
type Spec = schema.Spec

// Basic LDAP authentication service
type ldapSource struct {
	Addr   string      // LDAP address with host and port
	Base   string      // Base DN
	Domain string      // Domain of userPrincipalName
	BindDN string      // default reader dn
	Passwd string      // reader passwd
	cp     pool.Pooler // conn
	isAD   bool
}

// vars
var (
	ErrEmptyAddr = errors.New("ldap addr is empty")
	ErrEmptyBase = errors.New("ldap base is empty")
	ErrEmptyCN   = errors.New("ldap cn is empty")
	ErrEmptyDN   = errors.New("ldap dn is empty")
	ErrEmptyPwd  = errors.New("ldap passwd is empty")
	ErrLogin     = errors.New("Incorrect Username/Password")
	ErrNotFound  = errors.New("Not Found")
	ErrUnsupport = errors.New("Unsupported")
	userDnFmt    = "uid=%s,ou=people,%s"

	once sync.Once

	debug = godebug.Debug("staffio:ldap")
)

// newSource Add a new source (LDAP directory) to the global pool
func newSource(cfg *Config) (*ldapSource, error) {

	log.Printf("new source %s", cfg.Addr)

	u, err := url.Parse(cfg.Addr)
	if err != nil {
		return nil, fmt.Errorf("parse LDAP addr ERR: %s", err)
	}

	if u.Host == "" && u.Path != "" {
		u.Host = u.Path
		u.Path = ""
	}

	var useSSL bool
	if u.Scheme == "ldaps" {
		useSSL = true
	}

	pos := last(u.Host, ':')
	if pos < 0 {
		if useSSL {
			u.Host = u.Host + ":636"
		} else {
			u.Host = u.Host + ":389"
		}
	}

	opt := &pool.Options{
		Factory: func() (ldap.Client, error) {
			debug("dial to %s", u.Host)
			if useSSL {
				return ldap.DialTLS("tcp", u.Host, &tls.Config{InsecureSkipVerify: true})
			}
			return ldap.Dial("tcp", u.Host)
		},
		PoolSize:           DefaultPoolSize,
		PoolTimeout:        30 * time.Second,
		MaxConnAge:         25 * time.Minute,
		IdleTimeout:        5 * time.Minute,
		IdleCheckFrequency: 2 * time.Minute,
	}

	ls := &ldapSource{
		Addr:   u.Host,
		Base:   cfg.Base,
		Domain: cfg.Domain,
		BindDN: cfg.Bind,
		Passwd: cfg.Passwd,
		cp:     pool.NewPool(opt),
	}

	return ls, nil
}

func (ls *ldapSource) Close() {
	if ls.cp != nil {
		ls.cp.Close()
	}
}

func (ls *ldapSource) UDN(uid string) string {
	if ls.isAD {
		return etADuser.DN(uid, ls.Base)
	}
	return etPeople.DN(uid, ls.Base)
}

func (ls *ldapSource) Ready(names ...string) (err error) {
	err = ls.opWithMan(func(c ldap.Client) (err error) {
		for _, name := range names {
			if name == "" {
				continue
			}
			if name == "base" {
				var exist *ldap.Entry
				exist, err = ldapEntryReady(c, etBase, splitDC(ls.Base), ls.Base)
				if err == nil && exist != nil {
					once.Do(func() {
						if exist.GetAttributeValue("instanceType") != "" {
							debug("The source is Active Directory!")
							ls.isAD = true
						}
					})
				}
			} else if !ls.isAD {
				_, err = ldapEntryReady(c, etParent, name, ls.Base)
			}
		}
		return
	})
	return
}

func ldapEntryReady(c ldap.Client, et *entryType, name, base string) (exist *ldap.Entry, err error) {
	dn := et.DN(name, base)
	exist, err = ldapEntryGet(c, dn, et.Filter, et.Attributes...)
	debug("check ready for %s done, ERR %v", name, err)
	if err == ErrNotFound {
		ar := ldap.NewAddRequest(dn, nil)
		// ar.Attribute("objectClass", []string{et.OC, "top"})
		// ar.Attribute(et.PK, []string{name})
		et.prepareTo(name, ar)
		debug("add %v", ar)
		err = c.Add(ar)
		if err != nil {
			debug("add %q, ERR: %s", dn, err)
		} else {
			debug("add %q OK", dn)
		}
		return
	}
	return
}

func ldapEntryGet(c ldap.Client, dn, filter string, attrs ...string) (*ldap.Entry, error) {
	search := ldap.NewSearchRequest(
		dn,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		filter,
		attrs,
		nil)
	sr, err := c.Search(search)
	if err == nil {
		if len(sr.Entries) > 0 {
			debug("found dn %q entries %d", dn, len(sr.Entries))
			return sr.Entries[0], nil
		}
		debug("search %s with filter %s, not found", dn, filter)
		return nil, ErrNotFound
	}

	debug("ldap search %q, ERR: %s", dn, err)
	if le, ok := err.(*ldap.Error); ok && le.ResultCode == ldap.LDAPResultNoSuchObject {
		return nil, ErrNotFound
	}
	log.Printf("LDAP Search '%s' Error: %s", dn, err)
	return nil, err
}

// Authenticate
func (ls *ldapSource) Authenticate(uid, passwd string) (staff *People, err error) {
	var entry *ldap.Entry
	dn := ls.UDN(uid)
	entry, err = ls.bind(uid, dn, passwd, false)
	debug("authenticate(%q) %q ERR %v", dn, ls.Domain, err)
	if err == ErrLogin && ls.isAD && ls.Domain != "" && !strings.Contains(uid, "@") {
		dn = uid + "@" + ls.Domain
		entry, err = ls.bind(uid, dn, passwd, true)
	}
	if err == nil {
		staff = entryToPeople(entry)
	}
	return
}

func (ls *ldapSource) bind(uid, dn, passwd string, isUPN bool) (entry *ldap.Entry, err error) {
	var et *entryType
	if ls.isAD {
		et = etADuser
	} else {
		et = etPeople
	}
	err = ls.opWithDN(dn, passwd, func(c ldap.Client) (err error) {
		if !isUPN {
			entry, err = ldapEntryGet(c, dn, et.Filter, et.Attributes...)
		} else {
			entry, err = ldapEntryGet(c, ls.Base, "(userPrincipalName="+dn+")", et.Attributes...)
			if err == ErrNotFound {
				entry, err = ldapEntryGet(c, ls.Base, "(sAMAccountName="+uid+")", et.Attributes...)
			}
		}

		return
	})
	if err != nil {
		log.Printf("LDAP Bind failed for %s, reason: %s", dn, err)
		if le, ok := err.(*ldap.Error); ok {
			if le.ResultCode == ldap.LDAPResultInvalidCredentials ||
				le.ResultCode == ldap.LDAPResultInvalidDNSyntax {
				err = ErrLogin
				return
			}
		}
		return
	}

	debug("bind(%s, ***) ok", dn)
	return
}

type opFunc func(c ldap.Client) error

// opWithMan admin operate
func (ls *ldapSource) opWithMan(op opFunc) error {
	return ls.opWithDN(ls.BindDN, ls.Passwd, op)
}

func (ls *ldapSource) opWithDN(dn, passwd string, op opFunc) error {
	if dn == "" {
		return ErrEmptyDN
	}
	if passwd == "" {
		return ErrEmptyPwd
	}
	c, err := ls.cp.Get()
	if err == nil {
		defer ls.cp.Put(c)
		err = c.Bind(dn, passwd)
		if err == nil {
			debug("bind(%s) OK @ %s (len %d, idle %d)", dn, ls.Addr, ls.cp.Len(), ls.cp.IdleLen())
			return op(c)
		}
		log.Printf("bind(%s) ERR %s", dn, err)
		return err
	}

	log.Printf("get LDAP client from pool error, %s:%v", ls.Addr, err)
	return err
}

func (ls *ldapSource) getGroupEntry(cn string) (*ldap.Entry, error) {
	if len(cn) == 0 {
		return nil, ErrEmptyCN
	}
	if ls.isAD {
		if cn == groupAdminDefault {
			cn = groupAdminAD
		}
		return ls.getEntry(etADgroup.DN(cn, ls.Base), etADgroup.Filter, etADgroup.Attributes...)
	}
	return ls.getEntry(etGroup.DN(cn, ls.Base), etGroup.Filter, etGroup.Attributes...)
}

func (ls *ldapSource) getPeopleEntry(uid string) (*ldap.Entry, error) {
	if ls.isAD {
		return ls.getEntry(etADuser.DN(uid, ls.Base), etADuser.Filter, etADuser.Attributes...)
	}
	return ls.getEntry(etPeople.DN(uid, ls.Base), etPeople.Filter, etPeople.Attributes...)
}

// Entry return a special entry with dn and filter
func (ls *ldapSource) getEntry(dn, filter string, attrs ...string) (*ldap.Entry, error) {
	var entry *ldap.Entry
	err := ls.opWithMan(func(c ldap.Client) (err error) {
		entry, err = ldapEntryGet(c, dn, filter, attrs...)
		return
	})
	return entry, err
}

// GetPeople : search an LDAP source if an entry (with uid) is valide and in the specific filter
func (ls *ldapSource) GetPeople(uid string) (staff *People, err error) {
	var entry *ldap.Entry
	entry, err = ls.getPeopleEntry(uid)
	if err != nil {
		log.Printf("GetPeople(%s) ERR %s", uid, err)
		return nil, err
	}

	return entryToPeople(entry), nil
}

func (ls *ldapSource) GetByDN(dn string) (staff *People, err error) {
	var et *entryType
	if ls.isAD {
		et = etADuser
	} else {
		et = etPeople
	}
	var entry *ldap.Entry
	entry, err = ls.getEntry(dn, et.Filter, et.Attributes...)
	if err == nil {
		staff = entryToPeople(entry)
	}
	return
}

// List search paged results
// see also: https://tools.ietf.org/html/rfc2696
func (ls *ldapSource) List(spec *Spec) (data Peoples) {
	var et *entryType
	if ls.isAD {
		et = etADuser
	} else {
		et = etPeople
	}
	filter := et.Filter
	if len(spec.UIDs) > 0 {
		var sb strings.Builder
		sb.WriteString("(&" + et.Filter)
		sb.WriteString("(|")
		for _, uid := range spec.UIDs {
			sb.WriteString("(uid=" + uid + ")")
		}
		sb.WriteString("))")
		filter = sb.String()
		debug("list filter %q", filter)
	} else if len(spec.Name) > 0 {
		filter = "(&" + et.Filter + "(cn=" + spec.Name + "))"
	} else if len(spec.Email) > 0 {
		filter = "(&" + et.Filter + "(mail=" + spec.Email + "))"
	} else if len(spec.Mobile) > 0 {
		filter = "(&" + et.Filter + "(mobile=" + spec.Mobile + "))"
	}
	// TODO: other spec

	search := ldap.NewSearchRequest(
		ls.Base,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		filter,
		et.Attributes,
		nil)

	var (
		sr *ldap.SearchResult
	)
	err := ls.opWithMan(func(c ldap.Client) (err error) {
		sr, err = c.SearchWithPaging(search, uint32(spec.Limit))
		return
	})
	if err != nil {
		log.Printf("ERROR: %s for search %v\n", err, search)
		return
	}

	if len(sr.Entries) > 0 {
		data = make(Peoples, len(sr.Entries))
		for i, entry := range sr.Entries {
			data[i] = *entryToPeople(entry)
		}
	}

	return
}

func entryToPeople(entry *ldap.Entry) (u *People) {
	u = &People{
		DN:           entry.DN,
		UID:          entry.GetAttributeValue("uid"),
		Surname:      entry.GetAttributeValue("sn"),
		GivenName:    entry.GetAttributeValue("givenName"),
		CommonName:   entry.GetAttributeValue("cn"),
		Email:        entry.GetAttributeValue("mail"),
		Nickname:     entry.GetAttributeValue("displayName"),
		Mobile:       entry.GetAttributeValue("mobile"),
		EmployeeType: entry.GetAttributeValue("employeeType"),
		Birthday:     entry.GetAttributeValue("dateOfBirth"),
		AvatarPath:   entry.GetAttributeValue("avatarPath"),
		Description:  entry.GetAttributeValue("description"),
		JoinDate:     entry.GetAttributeValue("dateOfJoin"),
		IDCN:         entry.GetAttributeValue("idcnNumber"),
	}
	if str := entry.GetAttributeValue("sAMAccountName"); str != "" && u.UID == "" {
		u.UID = str
	}
	if str := entry.GetAttributeValue("userPrincipalName"); str != "" && u.Email == "" {
		u.Email = str
	}
	if str := entry.GetAttributeValue("gender"); str != "" && u.Email == "" {
		u.Gender = str
	}

	var err error
	if str := entry.GetAttributeValue("employeeNumber"); str != "" {
		u.EmployeeNumber, err = strconv.Atoi(str)
		if err != nil {
			log.Printf("invalid employee number %q, ERR %s", str, err)
		}
	}

	var t time.Time
	if str := entry.GetAttributeValue("createdTime"); str != "" {
		t, err = time.Parse(TimeLayout, str)
		if err != nil {
			log.Printf("invalid time %s, ERR %s", str, err)
		} else {
			u.Created = &t
		}
	} else if str := entry.GetAttributeValue("createTimestamp"); str != "" {
		t, err = time.Parse(TimeLayout, str)
		if err != nil {
			log.Printf("invalid time %s, ERR %s", str, err)
		} else {
			u.Created = &t
		}
	}

	if str := entry.GetAttributeValue("modifiedTime"); str != "" {
		t, err = time.Parse(TimeLayout, str)
		if err != nil {
			log.Printf("invalid time %s, ERR %s", str, err)
		} else {
			u.Modified = &t
		}
	} else if str := entry.GetAttributeValue("modifyTimestamp"); str != "" {
		t, err = time.Parse(TimeLayout, str)
		if err != nil {
			log.Printf("invalid time %s, ERR %s", str, err)
		} else {
			u.Modified = &t
		}
	}
	if blob := entry.GetRawAttributeValue("jpegPhoto"); len(blob) > 0 {
		u.JpegPhoto = blob
	}
	return
}

// Index of rightmost occurrence of b in s.
func last(s string, b byte) int {
	i := len(s)
	for i--; i >= 0; i-- {
		if s[i] == b {
			break
		}
	}
	return i
}
