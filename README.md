# staffio-backend

Data storage of staffio with LDAP

[![Build Status](https://travis-ci.org/liut/staffio-backend.svg?branch=master)](https://travis-ci.org/liut/staffio-backend)
[![codecov](https://codecov.io/gh/liut/staffio-backend/branch/master/graph/badge.svg)](https://codecov.io/gh/liut/staffio-backend)

## Features

### People interface
* Save and update a People
* Modify by self or admin
* Change password by self or admin
* Delete a People
* Authenticate with UID and password
* Browse with paged

### Group interface
* Create a group
* Delete by admin
* Browse all group

## Objects

### People

```go
type People struct {
	UID            string
	CommonName     string
	GivenName      string
	Surname        string
	Nickname       string
	Birthday       string
	Gender         string
	Email          string
	Mobile         string
	Tel            string
	EmployeeNumber string
	EmployeeType   string
	AvatarPath     string
	JpegPhoto      []byte
	Description    string
	JoinDate       string
	IDCN           string

	Organization  string
	OrgDepartment string
}
```

### Group

```go
type Group struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Members     []string `json:"members"`
}
```

## Variables in environment

| Name       | Default value        | Note |
| ------------ | ------------------ | ---- |
| `LDAP_HOSTS`   | localhost          | LDAP server addresses starting with `ldap://` or `ldaps://`, separated by commas if there are multiple. |
| `LDAP_BASE`    | dc=mydomain,dc=net | `AD`/`LDAP` base, required |
| `LDAP_DOMAIN`  | mydomain.net       | Suffix of <abbr title="userPrincipalName">`UPN`</abbr>, recommend set it |
| `LDAP_BIND_DN` |                    | <abbr title="distinguishedName">`DN`</abbr> of LDAP Admin or other writable user |
| `LDAP_PASSWD`  |                    | Password of LDAP Admin or other writable user |


## Usage example

```go

import "github.com/liut/staffio-backend/ldap"

main () {
	cfg := ldap.NewConfig()

	store, err := ldap.NewStore(cfg)
	if err != nil {
		log.Fatalf("new ldap store ERR %s", err)
	}

	err = store.Ready()
	if err != nil {
		log.Fatalf("the store ready failed, ERR %s", err)
	}

	uid := "eagle"
	passwrod := "mypassword"

	people, err := store.Authenticate(uid, password)
	if err != nil {
		log.Fatalf("login failed, ERR %s", err)
	}

	log.Printf("welcome %s", people.Name())
}
