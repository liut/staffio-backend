package ldap

import (
	"strconv"
	"time"

	"github.com/go-ldap/ldap/v3"
)

func (ls *ldapSource) savePeople(staff *People) (isNew bool, err error) {
	err = ls.opWithMan(func(c ldap.Client) (err error) {
		var entry *ldap.Entry
		entry, err = ldapFindOne(c, ls.Base, etPeople.oneFilter(staff.UID), etPeople.Attributes...)
		if err == nil {
			// :update
			mr := makeModifyRequest(entry, staff)
			eidStr := strconv.Itoa(staff.EmployeeNumber)
			if staff.EmployeeNumber > 0 && eidStr != entry.GetAttributeValue("employeeNumber") {
				mr.Replace("employeeNumber", []string{eidStr})
			}
			if staff.EmployeeType != entry.GetAttributeValue("employeeType") {
				mr.Replace("employeeType", []string{staff.EmployeeType})
			}
			err = c.Modify(mr)
			if err != nil {
				logger().Infow("modify fail", "mr", mr, "err", err)
			}
			return
		}
		if err == ErrNotFound {
			dn := ls.UDN(staff.UID)
			isNew = true
			ar := makeAddRequest(dn, staff)
			err = c.Add(ar)
			if err != nil {
				logger().Infow("add fail", "isAD", ls.isAD, "dn", dn, "staff", staff, "err", err)
			}
			return
		}
		logger().Infow("savePeople fail", "uid", staff.UID, "err", err)

		return
	})

	return
}

func makeAddRequest(dn string, staff *People) *ldap.AddRequest {
	ar := ldap.NewAddRequest(dn, nil)
	ar.Attribute("objectClass", objectClassPeople)
	ar.Attribute("uid", []string{staff.UID})
	ar.Attribute("cn", []string{staff.GetCommonName()})
	if staff.Surname != "" {
		ar.Attribute("sn", []string{staff.Surname})
	}
	if staff.GivenName != "" {
		ar.Attribute("givenName", []string{staff.GivenName})
	}

	if staff.Email != "" {
		ar.Attribute("mail", []string{staff.Email})
	}

	if staff.Nickname != "" {
		ar.Attribute("displayName", []string{staff.Nickname})
	}
	if staff.Mobile != "" {
		ar.Attribute("mobile", []string{staff.Mobile})
	}

	if staff.EmployeeNumber > 0 {
		ar.Attribute("employeeNumber", []string{strconv.Itoa(staff.EmployeeNumber)})
	}
	if staff.EmployeeType != "" {
		ar.Attribute("employeeType", []string{staff.EmployeeType})
	}
	if staff.Gender != "" {
		ar.Attribute("gender", []string{staff.Gender[0:1]})
	}
	if staff.Birthday != "" {
		ar.Attribute("dateOfBirth", []string{staff.Birthday})
	}
	if staff.Description != "" {
		ar.Attribute("description", []string{staff.Description})
	}
	if staff.AvatarPath != "" {
		ar.Attribute("avatarPath", []string{staff.AvatarPath})
	}
	if staff.JoinDate != "" {
		ar.Attribute("dateOfJoin", []string{staff.JoinDate})
	}
	if staff.Created != nil {
		ar.Attribute("createdTime", []string{staff.Created.Format(TimeLayout)})
	}

	// if staff.Passwd != "" {
	// 	ar.Attribute("userPassword", []string{staff.Passwd})
	// }

	return ar
}

func makeModifyRequest(entry *ldap.Entry, staff *People) *ldap.ModifyRequest {
	mr := ldap.NewModifyRequest(entry.DN, nil)
	mr.Replace("objectClass", objectClassPeople)
	if staff.Surname != entry.GetAttributeValue("sn") {
		mr.Replace("sn", []string{staff.Surname})
	}
	if staff.GivenName != entry.GetAttributeValue("givenName") {
		mr.Replace("givenName", []string{staff.GivenName})
	}
	if staff.CommonName != entry.GetAttributeValue("cn") {
		mr.Replace("cn", []string{staff.GetCommonName()})
	}
	if len(staff.Nickname) > 0 && staff.Nickname != entry.GetAttributeValue("displayName") {
		mr.Replace("displayName", []string{staff.Nickname})
	}
	if len(staff.Email) > 0 && staff.Email != entry.GetAttributeValue("mail") {
		mr.Replace("mail", []string{staff.Email})
	}
	if len(staff.Mobile) > 0 && staff.Mobile != entry.GetAttributeValue("mobile") {
		mr.Replace("mobile", []string{staff.Mobile})
	}
	if len(staff.AvatarPath) > 0 && staff.AvatarPath != entry.GetAttributeValue("avatarPath") {
		mr.Replace("avatarPath", []string{staff.AvatarPath})
	}
	if staff.Gender != "" {
		mr.Replace("gender", []string{staff.Gender[0:1]})
	}
	if len(staff.Birthday) > 0 && staff.Birthday != entry.GetAttributeValue("dateOfBirth") {
		mr.Replace("dateOfBirth", []string{staff.Birthday})
	}
	if len(staff.Description) > 0 && staff.Description != entry.GetAttributeValue("description") {
		mr.Replace("description", []string{staff.Description})
	}
	modified := time.Now()
	if staff.Modified != nil {
		modified = *staff.Modified
	}
	mr.Replace("modifiedTime", []string{modified.Format(TimeLayout)})

	return mr
}

/*
uid
sn
givenName
cn
mail
displayName
mobile
employeeNumber
employeeType
description
*/

// Rename change uid
func (ls *ldapSource) Rename(oldUID, newUID string) error {
	if 0 == len(oldUID) || 0 == len(newUID) {
		return ErrEmptyUID
	}
	return ls.opWithMan(func(c ldap.Client) (err error) {
		et := ls.etUser()
		var entry *ldap.Entry
		entry, err = ldapFindOne(c, ls.Base, et.oneFilter(oldUID), et.Attributes...)
		if err != nil {
			return
		}
		req := ldap.NewModifyDNRequest(entry.DN, et.PK+"="+newUID, true, "")
		if err = c.ModifyDN(req); err != nil {
			logger().Warnw("rename fail", "old", oldUID, "new", newUID, "err", err)
		}

		return
	})
}
