package ldap

import (
	"fmt"
	"github.com/ltkh/alertstrap/internal/config"
    "gopkg.in/ldap.v3"
)

func Search(conf *config.Ldap, username, password string) (*ldap.SearchResult, error) {

	var sr *ldap.SearchResult

	conn, err := ldap.DialURL(conf.Dial_url)
	if err != nil {
		return sr, err
	}
	defer conn.Close()

	if conf.Bind_user == "" && conf.Bind_pass == "" {
		conf.Bind_user = username
		conf.Bind_pass = password
	}

	err = conn.Bind(fmt.Sprintf(conf.Bind_dn, conf.Bind_user), conf.Bind_pass)
	if err != nil {
		return sr, err
	}

	searchRequest := ldap.NewSearchRequest(
		conf.Group_dn,
		ldap.ScopeWholeSubtree, 
		ldap.NeverDerefAliases, 
		0, 
		0, 
		false,
		fmt.Sprintf(conf.Filter_dn, username),
		[]string{"givenName", "sn", "mail", "uid"},
		nil,
	)

	sr, err = conn.Search(searchRequest)
	if err != nil {
		return sr, err
	}

	if len(sr.Entries) != 1 {
		return sr, fmt.Errorf("User not find or too many. count=%d", len(sr.Entries))
	}

	//userdn := sr.Entries[0].DN

	return sr, nil

}