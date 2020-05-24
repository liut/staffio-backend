package ldap

import (
	"log"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/stretchr/testify/assert"

	zlog "github.com/liut/staffio-backend/log"
	"github.com/liut/staffio-backend/schema"
)

var (
	cfg   *Config
	store *Store
)

func TestMain(m *testing.M) {
	_logger, _ := zap.NewDevelopment()
	defer _logger.Sync() // flushes buffer, if any
	sugar := _logger.Sugar()
	zlog.SetLogger(sugar)

	log.SetFlags(log.Ltime | log.Lshortfile)

	var err error

	cfg = NewConfig()
	cfg.Addr = envOr("TEST_LDAP_HOSTS", envOr("LDAP_HOSTS", "ldap://localhost"))
	cfg.Base = envOr("TEST_LDAP_BASE", envOr("LDAP_BASE", "dc=example,dc=org"))
	cfg.Domain = envOr("TEST_LDAP_DOMAIN", envOr("LDAP_DOMAIN", "example.org"))
	cfg.Bind = envOr("TEST_LDAP_BIND_DN", envOr("LDAP_BIND_DN", "cn=admin,dc=example,dc=org"))
	cfg.Passwd = envOr("TEST_LDAP_PASSWD", envOr("LDAP_PASSWD", "mypassword"))

	logger().Debugw("start test", "base", cfg.Base, "bind", cfg.Bind)

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
	_, err = NewStore(zeroConfig)
	assert.Error(t, err)
	assert.EqualError(t, err, ErrEmptyBase.Error())

	_s, err = NewStore(&Config{
		Addr: "ldaps://localhost",
		Base: cfg.Base,
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
	if assert.NoError(t, err) {
		assert.Equal(t, cn, staff.CommonName)
		assert.Equal(t, sn, staff.Surname)
	}

	staff2 := schema.NewPeople("cat", "cat", "cat")
	now := time.Now()
	staff2.Created = &now
	_, err = store.Save(staff2)
	assert.NoError(t, err)
	_, err = store.Get("cat")
	assert.NoError(t, err)

	specs := []*Spec{
		&Spec{UIDs: schema.UIDs{uid, staff2.UID}},
		&Spec{UIDs: schema.UIDs{uid}},
		&Spec{Name: cn},
		&Spec{Email: staff.Email},
		&Spec{Mobile: staff.Mobile},
	}

	for _, spec := range specs {
		data := store.All(spec)
		assert.NotZero(t, len(data))
	}

	err = store.PasswordReset(uid, password)
	assert.NoError(t, err)

	var _s *People
	_s, err = store.Authenticate(uid, password)
	if assert.NoError(t, err) {
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
	}

	err = store.PasswordChange(uid, "bad", "bad new")
	assert.Error(t, err)

	err = store.PasswordChange(uid, password, "secretNew")
	assert.NoError(t, err)

	err = store.Delete(uid)
	assert.NoError(t, err)
}

func TestRename(t *testing.T) {
	uid := "uid1"
	staff := schema.NewPeople(uid, "test1")
	isNew, err := store.Save(staff)
	assert.NoError(t, err)
	assert.True(t, isNew)

	staff, err = store.Get(uid)
	assert.NoError(t, err)
	assert.NotNil(t, staff)

	err = store.Rename(uid, "invalid + uid")
	assert.Error(t, err)

	newUID := "uid2"
	err = store.Rename(uid, newUID)
	assert.NoError(t, err)

	staff, err = store.Get(newUID)
	assert.NoError(t, err)
	assert.NotNil(t, staff)

	err = store.Delete(newUID)
	assert.NoError(t, err)
}

func TestGroup(t *testing.T) {
	var err error
	_, err = store.GetGroup("")
	assert.Error(t, err)
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

	err = ls.Delete(etParent.DN(name, cfg.Base))
	assert.NoError(t, err)
}

func TestStoreStats(t *testing.T) {
	t.Logf("stats: %v", store.PoolStats())
}
