package ldap

import (
	"fmt"
	"github.com/ltkh/alertstrap/internal/config"
    "gopkg.in/ldap.v3"
)

type Ldap struct {
	Conn   *ldap.Conn
	Conf   *config.Ldap
}

func New(conf *config.Ldap) (*Ldap, error) {
	conn, err := ldap.DialURL(conf.Dial_url)
	if err != nil {
		return nil, err
	}
	return &Ldap{ Conn: conn, Conf: conf }, nil
}

func (ld *Ldap) Search(username string, password string) (string, error) {

	if ld.Conf.Bind_user == "" && ld.Conf.Bind_pass == "" {
		ld.Conf.Bind_user = username
		ld.Conf.Bind_pass = password
	}

	err := ld.Conn.Bind(ld.Conf.Bind_user, ld.Conf.Bind_pass)
	if err != nil {
		return "", err
	}
	searchRequest := ldap.NewSearchRequest(
		ld.Conf.Group_dn,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf(ld.Conf.Filter_dn, username),
		[]string{"dn"},
		nil,
	)
	sr, err := ld.Conn.Search(searchRequest)
	if err != nil {
		return "", err
	}
	if len(sr.Entries) != 1 {
		return "", fmt.Errorf("user not find or too many. count=%d", len(sr.Entries))
	}
	userdn := sr.Entries[0].DN

	return userdn, nil

}