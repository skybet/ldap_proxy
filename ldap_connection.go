package main

// original work imported from https://github.com/jtblin/go-ldap-client (thank you)
// code is slightly changed

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log"

	ldap "gopkg.in/ldap.v2"
)

// LDAPConfiguration contains needed information to make ldap queries
type LDAPConfiguration struct {
	Attributes         []string
	Base               string
	BindDN             string
	BindPassword       string
	GroupFilter        string // e.g. "(memberUid=%s)"
	Host               string
	ServerName         string
	UserFilter         string // e.g. "(uid=%s)"
	Port               int
	InsecureSkipVerify bool
	UseTLS             bool
	ClientCertificates []tls.Certificate // Adding client certificates
}

// LDAPClient contains an LDAP connection
type LDAPClient struct {
	conn *ldap.Conn
	cfg  *LDAPConfiguration
}

// NewLDAPClient creates a connection to the ldap backend.
func NewLDAPClient(lc *LDAPConfiguration) (*LDAPClient, error) {
	l, err := ldap.Dial("tcp", fmt.Sprintf("%s:%d", lc.Host, lc.Port))
	if err != nil {
		log.Printf("Unable to connect to LDAP Server: %+v", err)
		return &LDAPClient{}, err
	}

	if lc.UseTLS {
		err = l.StartTLS(&tls.Config{InsecureSkipVerify: lc.InsecureSkipVerify})
		if err != nil {
			log.Printf("Unable to connect to LDAP Server with TLS: %+v", err)
		}
	}

	conn := LDAPClient{
		conn: l,
		cfg:  lc,
	}

	return &conn, err
}

// Close ldap connection
func (c *LDAPClient) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

// Authenticate authenticates the user against the ldap backend.
func (c *LDAPClient) Authenticate(username, password string) (bool, map[string]string, error) {
	if username == "" || password == "" {
		return false, nil, errors.New("invalid user or password")
	}

	// First bind with a read only user
	if c.cfg.BindDN != "" && c.cfg.BindPassword != "" {
		err := c.conn.Bind(c.cfg.BindDN, c.cfg.BindPassword)
		if err != nil {
			return false, nil, err
		}
	}

	attributes := append(c.cfg.Attributes, "dn")
	// Search for the given username
	searchRequest := ldap.NewSearchRequest(
		c.cfg.Base,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf(c.cfg.UserFilter, username),
		attributes,
		nil,
	)

	sr, err := c.conn.Search(searchRequest)
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
	user := map[string]string{
		"dn": userDN,
	}
	for _, attr := range c.cfg.Attributes {
		user[attr] = sr.Entries[0].GetAttributeValue(attr)
	}

	// Bind as the user to verify their password
	err = c.conn.Bind(userDN, password)
	if err != nil {
		return false, user, err
	}

	// Rebind as the read only user for any further queries
	if c.cfg.BindDN != "" && c.cfg.BindPassword != "" {
		err = c.conn.Bind(c.cfg.BindDN, c.cfg.BindPassword)
		if err != nil {
			return false, user, err
		}
	}

	return true, user, nil
}

// GetGroupsOfUser returns the group for a user.
func (c *LDAPClient) GetGroupsOfUser(username string) ([]string, error) {
	searchRequest := ldap.NewSearchRequest(
		c.cfg.Base,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf(c.cfg.GroupFilter, username),
		[]string{"cn"}, // can it be something else than "cn"?
		nil,
	)

	sr, err := c.conn.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	groups := []string{}
	for _, entry := range sr.Entries {
		groups = append(groups, entry.GetAttributeValue("cn"))
	}

	return groups, nil
}
