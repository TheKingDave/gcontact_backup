package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/emersion/go-vcard"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/people/v1"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
)

// Retrieves a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

// Requests a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(oauth2.NoContext, authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	defer f.Close()
	if err != nil {
		return nil, err
	}
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	defer f.Close()
	if err != nil {
		log.Fatalf("Unable to cache OAuth token: %v", err)
	}
	json.NewEncoder(f).Encode(token)
}

func googleToCard(p *people.Person, groupMap *map[string]string) (vcard.Card, error) {
	if len(p.Names) < 1 {
		return nil, errors.New("person has no name")
	}
	if groupMap == nil {
		return nil, errors.New("groupMap is nil")
	}

	card := make(vcard.Card)
	card.AddValue(vcard.FieldVersion, "3.0")

	// Create name
	pName := p.Names[0]
	name := vcard.Name{
		FamilyName:      pName.FamilyName,
		GivenName:       pName.GivenName,
		AdditionalName:  pName.MiddleName,
		HonorificPrefix: pName.HonorificPrefix,
		HonorificSuffix: pName.HonorificSuffix,
	}
	card.AddName(&name)
	card.AddValue(vcard.FieldFormattedName, pName.DisplayName)

	for _, m := range p.Memberships {
		card.AddValue(vcard.FieldCategories, (*groupMap)[m.ContactGroupMembership.ContactGroupResourceName])
	}

	for _, e := range p.EmailAddresses {
		params := make(vcard.Params)
		params.Add(vcard.ParamType, "INTERNET")
		params.Add(vcard.ParamType, strings.ToUpper(e.Type))

		card.Add(vcard.FieldEmail, &vcard.Field{
			Value:  e.Value,
			Params: params,
		})
	}

	for _, p := range p.PhoneNumbers {
		params := make(vcard.Params)
		params.Add(vcard.ParamType, strings.ToUpper(p.Type))

		card.Add(vcard.FieldTelephone, &vcard.Field{
			Value:  p.CanonicalForm,
			Params: params,
		})
	}

	for _, b := range p.Birthdays {
		card.AddValue(vcard.FieldBirthday, fmt.Sprintf("%04d%02d%02d", b.Date.Year, b.Date.Month, b.Date.Day))
	}

	for _, a := range p.Addresses {
		field := new(vcard.Field)
		field.Params = make(vcard.Params)
		if a.Type != "" {
			field.Params.Add(vcard.ParamType, strings.ToUpper(a.Type))
		}

		card.AddAddress(&vcard.Address{
			Field: field,

			PostOfficeBox:   a.PoBox,
			ExtendedAddress: a.ExtendedAddress,
			StreetAddress:   a.StreetAddress,
			Locality:        a.City,
			Region:          a.Region,
			PostalCode:      a.PostalCode,
			Country:         a.Country,
		})
	}

	title := ""

	for _, o := range p.Organizations {
		org := o.Name

		if o.Department != "" {
			org += ";" + o.Department
		}

		card.AddValue(vcard.FieldOrganization, org)

		if title == "" {
			title = o.Title
		}
	}

	if title != "" {
		card.AddValue(vcard.FieldTitle, title)
	}

	for _, u := range p.Urls {
		params := make(vcard.Params)
		params.Add(vcard.ParamType, strings.ToUpper(u.Type))

		card.Add(vcard.FieldURL, &vcard.Field{
			Value:  u.Value,
			Params: params,
		})
	}

	for _, i := range p.Photos {
		card.AddValue(vcard.FieldPhoto, i.Url)
	}

	for _, n := range p.Biographies {
		card.AddValue(vcard.FieldNote, n.Value)
	}

	return card, nil
}

func main() {
	b, err := ioutil.ReadFile("credentials_al.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, "https://www.googleapis.com/auth/contacts.readonly")
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(config)

	srv, err := people.New(client)
	if err != nil {
		log.Fatalf("Unable to retrieve People client: %v", err)
	}

	groupMap := make(map[string]string)

	groups, err := srv.ContactGroups.List().PageSize(1000).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve contact groups: %v", err)
	}
	for _, g := range groups.ContactGroups {
		groupMap[g.ResourceName] = g.FormattedName
		//fmt.Printf("Group: %s %s (%d)\n", g.Name, g.ResourceName, g.MemberCount)
	}
	fmt.Printf("Found %d groups\n", len(groups.ContactGroups))

	f, err := os.Create("cards.vcf")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	out := NewEncoder(f)

	personFields := "names,emailAddresses,phoneNumbers,birthdays,addresses,organizations,urls,photos,biographies,memberships"

	// Aaron Längert people/c7257551446154144875

	// Marie Aichagui people/c4186868436847727844
	// David Langheiter people/c1379591337294275221

	/*
	aaron, err := srv.People.
		Get("people/c4186868436847727844").
		PersonFields(personFields).
		Do()

	if err != nil {
		log.Fatalf("Unable to retrieve contact: %v", err)
	}

	card, _ := googleToCard(aaron, &groupMap)
	out.Encode(card)

	/**/


	var pageToken string
	totalPeople := 0

	// do, while -> Get all pages of People
	for {
		resp, err := srv.People.Connections.
			List("people/me").
			PersonFields(personFields).
			Sources("READ_SOURCE_TYPE_CONTACT").
			PageToken(pageToken).
			PageSize(500).
			Do()

		if err != nil {
			log.Fatalf("Unable to retrieve contacts: %v", err)
		}

		// Go trough all contacts and print them
		for _, p := range resp.Connections {
			if len(p.Names) < 1 {
				continue
			}

			card, err := googleToCard(p, &groupMap)
			if err != nil {
				fmt.Printf("Error %v", err)
				continue
			}

			err = out.Encode(card)
			if err != nil {
				fmt.Printf("Error %v", err)
				continue
			}
			totalPeople += 1
		}

		pageToken = resp.NextPageToken

		// Exit if no more pages exist
		if pageToken == "" {
			break
		}
	}

	fmt.Printf("Found %d people in total\n", totalPeople)

	/**/

}

// An Encoder formats cards.
type Encoder struct {
	w io.Writer
}

// NewEncoder creates a new Encoder that writes cards to w.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w}
}

// Encode formats a card. The card must have a FieldVersion field.
func (enc *Encoder) Encode(c vcard.Card) error {
	begin := "BEGIN:VCARD\r\n"
	if _, err := io.WriteString(enc.w, begin); err != nil {
		return err
	}

	version := c.Get(vcard.FieldVersion)
	if version == nil {
		return errors.New("vcard: VERSION field missing")
	}
	if _, err := io.WriteString(enc.w, formatLine(vcard.FieldVersion, version)+"\r\n"); err != nil {
		return err
	}

	var keys []string
	for k := range c {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fields := c[k]
		if strings.EqualFold(k, vcard.FieldVersion) {
			continue
		}
		// Quickfix for multi value CATEGORIES
		if strings.EqualFold(k, vcard.FieldCategories) {
			if _, err := io.WriteString(enc.w, formatLineMultiValue(k, fields)+"\r\n"); err != nil {
				return err
			}
			continue
		}
		for _, f := range fields {
			if _, err := io.WriteString(enc.w, formatLine(k, f)+"\r\n"); err != nil {
				return err
			}
		}
	}

	end := "END:VCARD\r\n"
	_, err := io.WriteString(enc.w, end)
	return err
}

func formatLineMultiValue(key string, fields []*vcard.Field) string {
	var s string

	s += key

	vals := make([]string, len(fields))
	for i, f := range fields {
		vals[i] = formatValue(f.Value)
	}

	s += ":" + strings.Join(vals, ",")
	return s
}

func formatLine(key string, field *vcard.Field) string {
	var s string

	if field.Group != "" {
		s += field.Group + "."
	}
	s += key

	for pk, pvs := range field.Params {
		for _, pv := range pvs {
			s += ";" + formatParam(pk, pv)
		}
	}

	s += ":" + formatValue(field.Value)
	return s
}

func formatParam(k, v string) string {
	return k + "=" + formatValue(v)
}

var valueFormatter = strings.NewReplacer("\\", "\\\\", "\n", "\\n", ",", "\\,")

func formatValue(v string) string {
	return valueFormatter.Replace(v)
}
