package schema

import (
	"encoding/base64"
	"strings"
	"time"
)

var (
	cnFormat = "<gn> <sn>"

	avatarReplacer = strings.NewReplacer("/0", "/60")
)

// SetNameFormat ...
func SetNameFormat(s string) {
	cnFormat = s
}

// People employment for a person
type People struct {
	UID            string `json:"uid" form:"uid" binding:"required"`        // 登录名
	CommonName     string `json:"cn" form:"cn"`                             // 姓名（全名,多用在中文）
	GivenName      string `json:"gn" form:"gn" binding:"required"`          // 名 FirstName
	Surname        string `json:"sn" form:"sn" binding:"required"`          // 姓 LastName
	Nickname       string `json:"nickname,omitempty" form:"nickname"`       // 昵称
	Birthday       string `json:"birthday,omitempty" form:"birthday"`       // 生日
	Gender         string `json:"gender,omitempty" form:"gender"`           // 性别: M F U
	Email          string `json:"email" form:"email" binding:"required"`    // 邮箱
	Mobile         string `json:"mobile" form:"mobile" binding:"required"`  // 手机
	Tel            string `json:"tel,omitempty" form:"tel"`                 // 座机
	EmployeeNumber int    `json:"eid,omitempty" form:"eid"`                 // 员工编号
	EmployeeType   string `json:"etype,omitempty" form:"etitle"`            // 员工岗位
	AvatarPath     string `json:"avatarPath,omitempty" form:"avatar"`       // 头像
	JpegPhoto      []byte `json:"-" form:"-"`                               // jpegPhoto data
	Description    string `json:"description,omitempty" form:"description"` // 描述
	JoinDate       string `json:"joinDate,omitempty" form:"joinDate"`       // 加入日期
	IDCN           string `json:"idcn,omitempty" form:"idcn"`               // 身份证号

	Organization  string `json:"org,omitempty" form:"org"`   // 所属组织
	OrgDepartment string `json:"dept,omitempty" form:"dept"` // 所属组织的部门

	Created  *time.Time `json:"created,omitempty" form:"-"`  // 创建时间
	Modified *time.Time `json:"modified,omitempty" form:"-"` // 修改时间

	DN string `json:"dn" form:"-"` // distinguishedName of LDAP entry

}

// GetUID ...
func (u *People) GetUID() string {
	return u.UID
}

// GetName ...
func (u *People) GetName() string {
	return u.Name()
}

// Name return nickname or commonName or fullname or uid
func (u *People) Name() string {
	if u.Nickname != "" {
		return u.Nickname
	}

	if u.CommonName != "" {
		return u.CommonName
	}

	if u.Surname != "" && u.GivenName != "" {
		return formatCN(u.GivenName, u.Surname)
	}

	return u.UID
}

// GetCommonName ...
func (u *People) GetCommonName() string {
	if u.CommonName != "" {
		return u.CommonName
	}

	return formatCN(u.GivenName, u.Surname)
}

// AvatarURI make uri of avatar
func (u *People) AvatarURI() string {
	if len(u.AvatarPath) > 0 {
		s := u.AvatarPath
		if strings.HasSuffix(s, "/") {
			s = s + "0"
		}
		if strings.HasPrefix(s, "//") || strings.HasPrefix(s, "http") { // full uri
			return s
		}
		if strings.HasPrefix(s, "/bizmail") || strings.HasPrefix(s, "/wwhead") { // wechat avatar
			return "https://p.qlogo.cn" + avatarReplacer.Replace(s)
		}
		// TODO: show uri
		return s
	}
	if len(u.JpegPhoto) > 0 {
		return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(u.JpegPhoto)
	}
	return ""
}

func formatCN(gn, sn string) string {
	r := strings.NewReplacer("<gn>", gn, "<sn>", sn)
	return r.Replace(cnFormat)
}

// UIDs ...
type UIDs []string

// Has ...
func (z UIDs) Has(uid string) bool {
	for _, s := range z {
		if s == uid {
			return true
		}
	}
	return false
}

// Peoples ...
type Peoples []People

// WithUID ...
func (arr Peoples) WithUID(uid string) *People {
	for _, u := range arr {
		if u.UID == uid {
			return &u
		}
	}
	return nil
}

// NewPeople args: uid, cn, sn, gn, nickname
func NewPeople(args ...string) *People {
	argc := len(args)
	if 0 == argc {
		panic("empty args")
	}
	obj := &People{
		UID:        args[0],
		CommonName: args[0],
		Surname:    args[0][0:1],
	}
	if argc > 1 {
		obj.CommonName = args[1]
		if argc > 2 {
			obj.Surname = args[2]
			if argc > 3 {
				obj.GivenName = args[3]
				if argc > 4 {
					obj.Nickname = args[4]
				}
			}
		} else {
			obj.Surname = args[1][0:1]
		}
	}

	return obj
}
