package ldap

import (
	"github.com/go-ldap/ldap/v3"
)

// ModifyBySelf ...
func (s *Store) ModifyBySelf(uid, password string, staff *People) (err error) {
	for _, ls := range s.sources {
		err = ls.Modify(uid, password, staff)
		if err != nil {
			logger().Infow("Modify by self fail", "uid", uid, "err", err)
		}
	}
	return
}

func (ls *ldapSource) Modify(uid, password string, staff *People) error {

	logger().Debugw("modify start", "uid", uid, "staff", staff)

	userdn := ls.UDN(uid)
	return ls.opWithDN(userdn, password, func(c ldap.Client) (err error) {
		entry, err := ldapFindOne(c, userdn, etPeople.Filter, etPeople.Attributes...)
		if err != nil {
			return err
		}

		modify := makeModifyRequest(entry, staff)

		if err = c.Modify(modify); err != nil {
			logger().Infow("modify fail", "dn", userdn, "err", err)
		}
		logger().Debugw("modified ok", "dn", userdn)
		return nil
	})

}
