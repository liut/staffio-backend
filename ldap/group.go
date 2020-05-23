package ldap

import (
	"strings"

	"github.com/go-ldap/ldap/v3"
)

const (
	groupAdminDefault = "keeper"
	groupAdminAD      = "Administrators"
)

var (
	groupLimit = 20
)

// AllGroup ...
func (s *Store) AllGroup() (data []Group, err error) {
	for _, ls := range s.sources {
		data, err = ls.SearchGroup("")
		if err == nil {
			return
		}
	}
	return
}

// GetGroup ...
func (s *Store) GetGroup(name string) (group *Group, err error) {
	// debug("Search group %s", name)
	for _, ls := range s.sources {
		var entry *ldap.Entry
		entry, err = ls.getGroupEntry(name)
		if err == nil {
			group = entryToGroup(entry)
			return
		}
		logger().Infow("search group fail", "name", name, "addr", ls.Addr, "err", err)
	}
	logger().Debugw("group not found", "name", name)
	if err == nil {
		err = ErrNotFound
	}
	return
}

// SearchGroup ...
func (ls *ldapSource) SearchGroup(name string) (data []Group, err error) {
	var (
		dn string
	)
	var et *entryType
	if ls.isAD {
		et = etADgroup
	} else {
		et = etGroup
	}
	if name == "" { // all
		dn = ls.Base
	} else {
		dn = et.DN(name, ls.Base)
	}

	var sr *ldap.SearchResult
	err = ls.opWithMan(func(c ldap.Client) (err error) {
		search := ldap.NewSearchRequest(
			dn,
			ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
			et.Filter,
			et.Attributes,
			nil)
		sr, err = c.SearchWithPaging(search, uint32(groupLimit))
		return
	})

	if err != nil {
		logger().Infow("LDAP search group fail", "name", name, "err", err)
		return
	}

	if len(sr.Entries) > 0 {
		data = make([]Group, len(sr.Entries))
		for i, entry := range sr.Entries {
			g := entryToGroup(entry)
			data[i] = *g
		}
	}

	return
}

func entryToGroup(entry *ldap.Entry) (g *Group) {
	g = new(Group)
	for _, attr := range entry.Attributes {
		if attr.Name == "cn" || attr.Name == "name" {
			g.Name = attr.Values[0]
		} else if attr.Name == "member" {
			g.Members = make([]string, len(attr.Values))
			for j, _dn := range attr.Values {
				g.Members[j] = _dn[strings.Index(_dn, "=")+1 : strings.Index(_dn, ",")]
			}
		}
	}
	// debug("group %q", g)
	return
}

// SaveGroup ...
func (s *Store) SaveGroup(group *Group) error {
	for _, ls := range s.sources {
		err := ls.saveGroup(group)
		if err != nil {
			logger().Infow("saveGroup fail", "group", group, "err", err)
			return err
		}
	}
	return nil
}

func (ls *ldapSource) saveGroup(group *Group) error {
	if ls.isAD {
		return ErrUnsupport
	}
	err := ls.opWithMan(func(c ldap.Client) error {
		gdn := etGroup.DN(group.Name, ls.Base)
		var members []string
		for _, m := range group.Members {
			members = append(members, ls.UDN(m))
		}
		_, err := ldapFindOne(c, gdn, etGroup.Filter, etGroup.Attributes...)
		if err == nil { // update
			mr := ldap.NewModifyRequest(gdn, nil)
			mr.Replace("member", members)
			logger().Debugw("change group", "mr", mr)
			err = c.Modify(mr)
		}
		if err == ErrNotFound { // create
			ar := ldap.NewAddRequest(gdn, nil)
			etGroup.prepareTo(group.Name, ar)
			ar.Attribute("member", members)
			logger().Debugw("add group", "ar", ar)
			err = c.Add(ar)
		}
		if err != nil {
			logger().Infow("saveGroup fail", "group", group, "err", err)
		}

		return err
	})
	return err
}

// EraseGroup ...
func (s *Store) EraseGroup(name string) error {
	for _, ls := range s.sources {
		err := ls.eraseGroup(name)
		if err != nil {
			logger().Infow("eraseGroup fail", "name", name, "err", err)
			return err
		}
	}
	return nil
}

func (ls *ldapSource) eraseGroup(name string) error {
	if ls.isAD {
		return ErrUnsupport
	}
	err := ls.opWithMan(func(c ldap.Client) error {
		dr := ldap.NewDelRequest(etGroup.DN(name, ls.Base), nil)
		return c.Del(dr)
	})
	return err
}
