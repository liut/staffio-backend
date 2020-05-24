package ldap

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-ldap/ldap/v3"

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
	ErrEmptyAddr   = errors.New("ldap addr is empty")
	ErrEmptyBase   = errors.New("ldap base is empty")
	ErrEmptyCN     = errors.New("ldap cn is empty")
	ErrEmptyDN     = errors.New("ldap dn is empty")
	ErrEmptyFilter = errors.New("ldap filter is empty")
	ErrEmptyPwd    = errors.New("ldap passwd is empty")
	ErrEmptyUID    = errors.New("ldap uid is empty")
	ErrInvalidUID  = errors.New("ldap uid is invalid")
	ErrLogin       = errors.New("Incorrect Username/Password")
	ErrNotFound    = errors.New("Not Found")
	ErrUnsupport   = errors.New("Unsupported")

	userDnFmt = "uid=%s,ou=people,%s"

	once sync.Once
)

// newSource Add a new source (LDAP directory) to the global pool
func newSource(cfg *Config) (*ldapSource, error) {

	logger().Debugw("new source ", "addr", cfg.Addr)

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
			logger().Debugw("dial to ldap", "host", u.Host)
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
	return ls.etUser().DN(uid, ls.Base)
}

func (ls *ldapSource) etUser() *entryType {
	if ls.isAD {
		return etADuser
	}
	return etPeople
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
							logger().Infow("The source is Active Directory!")
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
	exist, err = ldapFindOne(c, dn, et.Filter, et.Attributes...)
	logger().Debugw("check ready", "name", name, "err", err)
	if err == ErrNotFound {
		ar := ldap.NewAddRequest(dn, nil)
		// ar.Attribute("objectClass", []string{et.OC, "top"})
		// ar.Attribute(et.PK, []string{name})
		et.prepareTo(name, ar)
		logger().Debugw("add", "ar", ar)
		err = c.Add(ar)
		if err != nil {
			logger().Infow("add fail", "dn", dn, "err", err)
		} else {
			logger().Infow("add OK", "dn", dn)
		}
		return
	}
	return
}

func ldapFindOne(c ldap.Client, baseDN, filter string, attrs ...string) (*ldap.Entry, error) {
	search := ldap.NewSearchRequest(
		baseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		filter,
		attrs,
		nil)
	sr, err := c.Search(search)
	if err == nil {
		count := len(sr.Entries)
		if count > 0 {
			if count > 1 {
				logger().Infow("found more then one entries", "baseDN", baseDN, "count", count)
			}

			return sr.Entries[0], nil
		}
		logger().Infow("search not found", "baseDN", baseDN, "filter", filter)
		return nil, ErrNotFound
	}

	if le, ok := err.(*ldap.Error); ok && le.ResultCode == ldap.LDAPResultNoSuchObject {
		logger().Debugw("ldap search not found", "baseDN", baseDN, "filter", filter, "err", le)
		return nil, ErrNotFound
	}
	logger().Debugw("ldap search fail", "baseDN", baseDN, "filter", filter, "err", err)
	return nil, err
}

// Authenticate
func (ls *ldapSource) Authenticate(uid, passwd string) (staff *People, err error) {
	var entry *ldap.Entry
	entry, err = ls.bind(uid, passwd)
	logger().Debugw("authenticate fail", "uid", uid, "domain", ls.Domain, "err", err)
	if err == nil {
		staff = entryToPeople(entry)
	}
	return
}

func (ls *ldapSource) bind(uid, passwd string) (entry *ldap.Entry, err error) {
	et := ls.etUser()
	dn := et.DN(uid, ls.Base)
	err = ls.opWithDN(dn, passwd, func(c ldap.Client) (err error) {
		entry, err = ldapFindOne(c, dn, et.Filter, et.Attributes...)
		return
	})

	if err == ErrNotFound && ls.isAD && ls.Domain != "" && !strings.Contains(uid, "@") {
		dn = uid + "@" + ls.Domain
		err = ls.opWithDN(dn, passwd, func(c ldap.Client) (err error) {
			entry, err = ldapFindOne(c, ls.Base, "(userPrincipalName="+dn+")", et.Attributes...)
			if err == ErrNotFound {
				entry, err = ldapFindOne(c, ls.Base, "(sAMAccountName="+uid+")", et.Attributes...)
			}
			return
		})
	}
	if err == ErrNotFound {
		err = ls.opWithMan(func(c ldap.Client) (err error) {
			entry, err = ldapFindOne(c, ls.Base, et.oneFilter(uid), et.Attributes...)
			err = ls.opWithDN(entry.DN, passwd, nil)
			return
		})
	}

	if err != nil {
		logger().Infow("LDAP Bind failed", "dn", dn, "err", err)
		if le, ok := err.(*ldap.Error); ok {
			if le.ResultCode == ldap.LDAPResultInvalidCredentials ||
				le.ResultCode == ldap.LDAPResultInvalidDNSyntax {
				err = ErrLogin
				return
			}
		}
		return
	}

	logger().Debugw("bind ok", "dn", dn)
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
			logger().Debugw("bind ok", "dn", dn, "addr", ls.Addr, "pool.len", ls.cp.Len(), "pool.idleLen", ls.cp.IdleLen())
			if op != nil {
				return op(c)
			}
			return nil
		}
		logger().Infow("bind fail", "dn", dn, "err", err)
		return err
	}

	logger().Infow("get LDAP client from pool error, %s:%v", ls.Addr, err)
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
		return ls.getEntry(ls.Base, etADgroup.oneFilter(cn), etADgroup.Attributes...)
	}
	return ls.getEntry(ls.Base, etGroup.oneFilter(cn), etGroup.Attributes...)
}

func (ls *ldapSource) getPeopleEntry(uid string) (*ldap.Entry, error) {
	et := ls.etUser()
	return ls.getEntry(ls.Base, et.oneFilter(uid), et.Attributes...)
}

// Entry return a special entry in baseDN and filter
func (ls *ldapSource) getEntry(baseDN, filter string, attrs ...string) (*ldap.Entry, error) {
	var entry *ldap.Entry
	err := ls.opWithMan(func(c ldap.Client) (err error) {
		entry, err = ldapFindOne(c, baseDN, filter, attrs...)
		return
	})
	return entry, err
}

// GetPeople : search an LDAP source if an entry (with uid) is valide and in the specific filter
func (ls *ldapSource) GetPeople(uid string) (staff *People, err error) {
	var entry *ldap.Entry
	entry, err = ls.getPeopleEntry(uid)
	if err != nil {
		logger().Infow("getPeopleEntry fail", "uid", uid, "err", err)
		return nil, err
	}

	return entryToPeople(entry), nil
}

func (ls *ldapSource) GetByDN(dn string) (staff *People, err error) {
	if _, err = ldap.ParseDN(dn); err != nil {
		return
	}
	et := ls.etUser()
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
	et := ls.etUser()
	filter := et.Filter
	if len(spec.UIDs) > 0 {
		if 1 == len(spec.UIDs) {
			filter = "(&(uid=" + ldap.EscapeFilter(spec.UIDs[0]) + ")" + et.Filter + ")"
		} else {
			var sb strings.Builder
			sb.WriteString("(&")
			sb.WriteString("(|")
			for _, uid := range spec.UIDs {
				sb.WriteString("(uid=" + ldap.EscapeFilter(uid) + ")")
			}
			sb.WriteString(")" + et.Filter + ")")
			filter = sb.String()
		}
	} else if len(spec.Name) > 0 {
		filter = "(&(cn=" + ldap.EscapeFilter(spec.Name) + ")" + et.Filter + ")"
	} else if len(spec.Email) > 0 {
		filter = "(&(mail=" + ldap.EscapeFilter(spec.Email) + ")" + et.Filter + ")"
	} else if len(spec.Mobile) > 0 {
		filter = "(&(mobile=" + ldap.EscapeFilter(spec.Mobile) + ")" + et.Filter + ")"
	}
	logger().Debugw("list", "filter", filter)
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
		logger().Infow("list fail", "search", search, "err", err)
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
		Gender:       entry.GetAttributeValue("gender"),
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

	var err error
	if str := entry.GetAttributeValue("employeeNumber"); str != "" {
		u.EmployeeNumber, _ = strconv.Atoi(str)
	}

	var t time.Time
	if str := entry.GetAttributeValue("createdTime"); str != "" {
		t, err = time.Parse(TimeLayout, str)
		if err != nil {
			logger().Infow("invalid time", "str", str, "err", err)
		} else {
			u.Created = &t
		}
	} else if str := entry.GetAttributeValue("createTimestamp"); str != "" {
		if t, err = time.Parse(TimeLayout, str); err == nil {
			u.Created = &t
		}
	}

	if str := entry.GetAttributeValue("modifiedTime"); str != "" {
		t, err = time.Parse(TimeLayout, str)
		if err != nil {
			logger().Infow("invalid time", "str", str, "err", err)
		} else {
			u.Modified = &t
		}
	} else if str := entry.GetAttributeValue("modifyTimestamp"); str != "" {
		if t, err = time.Parse(TimeLayout, str); err == nil {
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
