package ldap

import (
	zlog "github.com/liut/staffio-backend/log"
)

func logger() zlog.Logger {
	return zlog.GetLogger()
}
