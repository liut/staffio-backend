package ldap

import (
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/liut/staffio-backend/schema"
)

const (
	testBase   = "dc=example,dc=org"
	testDomain = "example.org"
	testBind   = "cn=admin,dc=example,dc=org"
	testPasswd = "mypassword"
)

var (
	store *Store
)

func TestMain(m *testing.M) {
	log.SetFlags(log.Ltime | log.Lshortfile)

	var err error

	cfg := NewConfig()
	cfg.Base = envOr("LDAP_BASE_DN", testBase)
	cfg.Domain = envOr("LDAP_DOMAIN", testDomain)
	cfg.Bind = envOr("LDAP_BIND_DN", testBind)
	cfg.Passwd = envOr("LDAP_PASSWD", testPasswd)

	store, err = NewStore(cfg)
	if err != nil {
		log.Fatalf("new store ERR %s", err)
	}
	err = store.Ready()
	if err != nil {
		log.Fatalf("store ready ERR %s", err)
	}
	defer store.Close()
	m.Run()
}

func TestStoreFailed(t *testing.T) {
	var err error
	var _s *Store
	_, err = NewStore(Config{})
	assert.Error(t, err)
	assert.EqualError(t, err, ErrEmptyBase.Error())

	_, err = NewStore(Config{Addr: ":bad", Base: testBase})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse")

	_s, err = NewStore(Config{
		Addr: "ldaps://localhost",
		Base: testBase,
	})
	assert.NoError(t, err)
	// log.Printf("ldap store: %s", _s)
	err = _s.Ready()
	assert.Error(t, err)
	_s.Close()
}

func TestPeopleError(t *testing.T) {
	var err error
	_, err = store.Get("noexist")
	assert.Error(t, err)
	assert.EqualError(t, err, ErrNotFound.Error())

	err = store.Delete("noexist")
	assert.Error(t, err)

	_, err = store.Save(&People{})
	assert.Error(t, err)
	_, err = store.Save(&People{UID: "six"})
	assert.Error(t, err)

	_, err = store.Authenticate("baduid", "badPwd")
	assert.Error(t, err)
	assert.EqualError(t, err, ErrLogin.Error())
}

func TestPeople(t *testing.T) {
	var err error
	uid := "doe"
	cn := "doe"
	sn := "doe"
	password := "secret"
	staff := &People{
		// required fields
		UID:        uid,
		CommonName: cn,
		Surname:    sn,

		// optional fields
		GivenName:      "fawn",
		AvatarPath:     "avatar.png",
		Description:    "It's me",
		Email:          "fawn@deer.cc",
		Nickname:       "tiny",
		Birthday:       "20120304",
		Gender:         "m",
		Mobile:         "13012341234",
		JoinDate:       time.Now().Format(DateLayout),
		EmployeeNumber: 001,
		EmployeeType:   "Engineer",
	}

	var isNew bool
	isNew, err = store.Save(staff)
	assert.NoError(t, err)
	assert.True(t, isNew)

	isNew, err = store.Save(staff)
	assert.NoError(t, err)
	assert.False(t, isNew)

	staff, err = store.Get(uid)
	assert.NoError(t, err)
	assert.Equal(t, cn, staff.CommonName)
	assert.Equal(t, sn, staff.Surname)

	var spec = &Spec{
		UIDs: schema.UIDs{uid},
	}
	data := store.All(spec)
	assert.NotZero(t, len(data))

	err = store.PasswordReset(uid, password)
	assert.NoError(t, err)

	var _s *People
	_s, err = store.Authenticate(uid, password)
	assert.NoError(t, err)
	assert.NotNil(t, _s)

	staff, err = store.GetByDN(_s.DN)
	assert.NoError(t, err)
	assert.NotNil(t, staff)

	staff.CommonName = "doe2"
	staff.GivenName = "fawn2"
	staff.Surname = "deer2"
	staff.AvatarPath = "avatar2.png"
	staff.Description = "It's me 2"
	staff.Email = "fawn2@deer.cc"
	staff.Nickname = "tiny2"
	staff.Birthday = "20120305"
	staff.Gender = "f"
	staff.Mobile = "13012345678"
	staff.EmployeeNumber = 002
	staff.EmployeeType = "Chief Engineer"
	err = store.ModifyBySelf(uid, password, staff)
	assert.NoError(t, err)

	err = store.PasswordChange(uid, "bad", "bad new")
	assert.Error(t, err)

	err = store.PasswordChange(uid, password, "secretNew")
	assert.NoError(t, err)

	err = store.Delete(uid)
	assert.NoError(t, err)
}

func TestGroup(t *testing.T) {
	var err error
	_, err = store.GetGroup("noexist")
	assert.Error(t, err)

	group := &Group{
		Name:    "testgroup",
		Members: []string{"doe"},
	}

	err = store.SaveGroup(group)
	assert.NoError(t, err)
	err = store.SaveGroup(group)
	assert.NoError(t, err)

	_g, _e := store.GetGroup(group.Name)
	assert.NoError(t, _e)
	assert.NotEmpty(t, _g.Members)

	_, err = store.AllGroup()
	assert.NoError(t, err)

	err = store.EraseGroup(group.Name)
	assert.NoError(t, err)
}

func TestReady(t *testing.T) {
	var err error
	name := "teams"
	ls := store.sources[0]
	err = ls.Ready("")
	assert.NoError(t, err)
	err = ls.Ready(name)
	assert.NoError(t, err)

	err = ls.Delete(etParent.DN(name, testBase))
	assert.NoError(t, err)
}

func TestStoreStats(t *testing.T) {
	t.Logf("stats: %v", store.PoolStats())
}
