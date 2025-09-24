package ldap

import (
    "fmt"
    "errors"
    "github.com/go-ldap/ldap/v3"
    "crypto/tls"
)

type LDAPClient struct {
    Attributes         []string
    Base               string
    BindDN             string
    BindPassword       string
    GroupFilter        string // e.g. "(memberUid=%s)"
    Host               string
    ServerName         string
    UserFilter         string // e.g. "(uid=%s)"      
    Port               int
    UseSSL             bool
    SkipTLS            bool
    ClientCertificates []tls.Certificate // Adding client certificates
}

// Connect connects to the ldap backend.
func (lc *LDAPClient) Connect() (*ldap.Conn, error) {
    //var conn *ldap.Conn
    //var err error
    address := fmt.Sprintf("%s:%d", lc.Host, lc.Port)
    config := &tls.Config{InsecureSkipVerify: true}

    if lc.UseSSL {
        config = &tls.Config{
            InsecureSkipVerify: lc.SkipTLS,
            ServerName:         lc.ServerName,
        }
        if lc.ClientCertificates != nil && len(lc.ClientCertificates) > 0 {
            config.Certificates = lc.ClientCertificates
        }
    }

    
    conn, err := ldap.DialTLS("tcp", address, config)
    if err != nil {
        return nil, err
    }

    // Reconnect with TLS
    //if lc.SkipTLS {
    //    err = conn.StartTLS(&tls.Config{InsecureSkipVerify: true})
    //    if err != nil {
    //        return nil, err
    //    }
    //}

    return conn, nil
}

// Close closes the ldap backend connection.
//func (conn *ldap.Conn) Close() {
//    conn.Close()
//}

// Authenticate authenticates the user against the ldap backend.
func (lc *LDAPClient) Authenticate(username, password string) (bool, map[string]string, error) {
    conn, err := lc.Connect()
    if err != nil {
        return false, nil, err
    }
    defer conn.Close()

    // First bind with a read only user
    if lc.BindDN != "" && lc.BindPassword != "" {
        err := conn.Bind(lc.BindDN, lc.BindPassword)
        if err != nil {
            return false, nil, err
        }
    }

    attributes := append(lc.Attributes, "dn")
    // Search for the given username
    searchRequest := ldap.NewSearchRequest(
        lc.Base,
        ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
        fmt.Sprintf(lc.UserFilter, username),
        attributes,
        nil,
    )

    sr, err := conn.Search(searchRequest)
    if err != nil {
        return false, nil, err
    }

    if len(sr.Entries) < 1 {
        return false, nil, errors.New("User does not exist")
    }

    if len(sr.Entries) > 1 {
        return false, nil, errors.New("Too many entries returned")
    }

    userDN := sr.Entries[0].DN
    user := map[string]string{}
    for _, attr := range lc.Attributes {
        user[attr] = sr.Entries[0].GetAttributeValue(attr)
    }

    // Bind as the user to verify their password
    err = conn.Bind(userDN, password)
    if err != nil {
        return false, user, err
    }

    // Rebind as the read only user for any further queries
    if lc.BindDN != "" && lc.BindPassword != "" {
        err = conn.Bind(lc.BindDN, lc.BindPassword)
        if err != nil {
            return false, user, err
        }
    }

    return true, user, nil
}

// GetGroupsOfUser returns the group for a user.
func (lc *LDAPClient) GetGroupsOfUser(username string) ([]string, error) {
    conn, err := lc.Connect()
    if err != nil {
        return nil, err
    }
    defer conn.Close()

    searchRequest := ldap.NewSearchRequest(
        lc.Base,
        ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
        fmt.Sprintf(lc.GroupFilter, username),
        []string{"cn"}, // can it be something else than "cn"?
        nil,
    )

    sr, err := conn.Search(searchRequest)
    if err != nil {
        return nil, err
    }

    groups := []string{}
    for _, entry := range sr.Entries {
        groups = append(groups, entry.GetAttributeValue("cn"))
    }

    return groups, nil
}