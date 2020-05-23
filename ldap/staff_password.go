package ldap

import (
	"github.com/go-ldap/ldap/v3"
)

// PasswordChange ...
func (s *Store) PasswordChange(uid, oldPasswd, newPasswd string) (err error) {
	for _, ls := range s.sources {
		err = ls.PasswordChange(uid, oldPasswd, newPasswd)
		if err != nil {
			break
		}
	}
	return
}

func (ls *ldapSource) PasswordChange(uid, oldPasswd, newPasswd string) error {
	userdn := ls.UDN(uid)
	c, err := ls.cp.Get()
	defer ls.cp.Put(c)
	if err == nil {
		pmr := ldap.NewPasswordModifyRequest(userdn, oldPasswd, newPasswd)
		_, err := c.PasswordModify(pmr)
		if err != nil {
			logger().Infow("PasswordModify fail", "uid", uid, "err", err)
			return err
		}
		logger().Infow("PasswordModify OK", "uid", uid)
	}

	return err
}

// PasswordReset ...
func (s *Store) PasswordReset(uid, passwd string) (err error) {
	for _, ls := range s.sources {
		err = ls.PasswordReset(uid, passwd)
		if err != nil {
			break
		}
	}
	return
}

// password reset by administrator
func (ls *ldapSource) PasswordReset(uid, newPasswd string) error {
	dn := ls.UDN(uid)
	return ls.opWithMan(func(c ldap.Client) error {
		passwordModifyRequest := ldap.NewPasswordModifyRequest(dn, "", newPasswd)
		_, err := c.PasswordModify(passwordModifyRequest)
		if err != nil {
			logger().Infow("PasswordReset fail", "uid", uid, "err", err)
			return err
		}
		logger().Infow("PasswordModify OK", "uid", uid)
		return nil
	})
}
