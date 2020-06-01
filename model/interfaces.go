package model

// Spec param of searching
type Spec struct {
	Name   string   `json:"name,omitempty"`
	Email  string   `json:"email"`
	Mobile string   `json:"mobile"`
	UIDs   []string `json:"uids,omitempty"`
	Limit  int      `json:"limit,omitempty"`
}

// PeopleStore Storage for People
type PeopleStore interface {
	// All browse from store, like LDAP
	All(spec *Spec) Peoples
	// Get with uid
	Get(uid string) (*People, error)
	// GetByDN with dn
	GetByDN(dn string) (*People, error)
	// Delete with uid
	Delete(uid string) error
	// Save add or update
	Save(people *People) (isNew bool, err error)
	// ModifyBySelf update by self
	ModifyBySelf(uid, password string, people *People) error
}

// PasswordStore Storage for Password
type PasswordStore interface {
	// Change password by self
	PasswordChange(uid, oldPassword, newPassword string) error
	// Reset password by administrator
	PasswordReset(uid, newPassword string) error
}

// Authenticator for Authenticate
type Authenticator interface {
	// Authenticate with uid and password
	Authenticate(uid, password string) (*People, error)
}

// GroupStore Storage for Group
type GroupStore interface {
	AllGroup() ([]Group, error)
	GetGroup(name string) (*Group, error)
	SaveGroup(group *Group) error
	EraseGroup(name string) error
}
