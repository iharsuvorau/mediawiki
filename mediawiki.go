// Package mediawiki is a wrapper around MediaWiki's API.
package mediawiki

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// UpdatePage updates the content of the page.
func UpdatePage(mwURI, title, markup, contentModel, loginName, loginPass, sectionTitle string) (bool, error) {
	u := fmt.Sprintf("%s/api.php?action=parse&format=json&page=%s&prop=sections", mwURI, title)
	resp, err := http.Get(u)
	if err != nil {
		return false, err
	}
	data := struct {
		Parse struct {
			Sections []struct {
				Level string
				Line  string
				Index string
			}
		}
	}{}
	if err = json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return false, err
	}
	resp.Body.Close()

	var sectionIndex = "new"
	// check for existing Publications section and assign the index
	for _, section := range data.Parse.Sections {
		if section.Level == "2" && section.Line == sectionTitle {
			sectionIndex = section.Index
		}
	}

	// add a header if the section exists, because markup will overwrite it
	if sectionIndex != "new" {
		markup = fmt.Sprintf("== %s ==\n\n%s", sectionTitle, markup)
	}

	// edit API call

	// login
	ok, cookies, err := Login(mwURI, loginName, loginPass)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, fmt.Errorf("login failed")
	}
	// edit token
	csrfToken, _, err := GetToken(mwURI, "csrf", cookies)
	// request composing
	u = fmt.Sprintf("%s/api.php?action=edit&format=json", mwURI)
	v := url.Values{}
	v.Set("bot", "1")
	//v.Set("nocreate", "1")
	v.Set("title", title)
	v.Set("section", sectionIndex)
	v.Set("sectiontitle", sectionTitle)
	v.Set("text", markup)
	v.Set("contentmodel", contentModel)
	v.Set("token", csrfToken)
	req, err := http.NewRequest("POST", u, strings.NewReader(v.Encode()))
	if err != nil {
		return false, err
	}
	req.Header.Set("Cookie", joinCookies(cookies))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	// firing
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode > 200 {
		return false, fmt.Errorf("bad HTTP status")
	}

	// interpreting results, extract errors and the success message
	{
		data := map[string]interface{}{}
		if err = json.NewDecoder(resp.Body).Decode(&data); err != nil {
			return false, err
		}
		if s, ok := data["error"].(string); ok && len(s) > 0 {
			return false, fmt.Errorf("UpdatePage: error response: %+v", s)
		}
		editData, ok := data["edit"].(map[string]interface{})
		if !ok {
			return false, fmt.Errorf("UpdatePage: unexpected response: %v", data)
		}
		if s, ok := editData["result"].(string); ok && s != "Success" {
			err = fmt.Errorf("UpdatePage: error response: %+v", data)
			return false, err
		}
	}

	return true, nil
}

// GetToken returns a token, auth cookies and error.
func GetToken(mwURI, tokenType string, cookies []*http.Cookie) (string, []*http.Cookie, error) {
	u := fmt.Sprintf("%s/api.php?action=query&format=json&meta=tokens&type=%s", mwURI, tokenType)

	req, err := http.NewRequest("POST", u, nil)
	if err != nil {
		return "", nil, err
	}

	if cookies != nil {
		req.Header.Set("Cookie", joinCookies(cookies))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	data := map[string]interface{}{}

	if err = json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", nil, err
	}

	query, ok := data["query"].(map[string]interface{})
	if !ok {
		return "", nil, fmt.Errorf("GetToken: query type error: %v", data)
	}

	tokens, ok := query["tokens"].(map[string]interface{})
	if !ok {
		return "", nil, fmt.Errorf("GetToken: tokens type error: %v", data)
	}

	var token string
	switch tokenType {
	case "login":
		token = tokens["logintoken"].(string)
		break
	case "csrf":
		token = tokens["csrftoken"].(string)
		break
	case "userrights":
		token = tokens["userrightstoken"].(string)
		break
	default:
		return "", nil, fmt.Errorf("GetToken: token type is not recognized: %v", tokenType)
	}

	if len(token) == 0 {
		return "", nil, fmt.Errorf("GetToken: zero length token: %s", data)
	}

	return token, resp.Cookies(), nil
}

// Login logins a user given a name and pass and returns a success status, auth cookies and error.
func Login(mwURI, name, pass string) (bool, []*http.Cookie, error) {
	var ok bool

	// getting a login token and cookies
	token, cookies, err := GetToken(mwURI, "login", nil)
	if err != nil {
		return ok, nil, err
	}

	// preparing a request

	u := fmt.Sprintf("%s/api.php", mwURI)
	v := url.Values{}
	v.Set("action", "login")
	v.Set("format", "json")
	v.Set("lgname", name)
	v.Set("lgpassword", pass)
	v.Set("lgtoken", token)

	req, err := http.NewRequest("POST", u, strings.NewReader(v.Encode()))
	if err != nil {
		return ok, nil, err
	}

	req.Header.Add("Cookie", joinCookies(cookies))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	// firing and decoding results

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ok, nil, err
	}
	defer resp.Body.Close()

	// interpreting results, extract errors and the success message
	// using a map
	data := map[string]interface{}{}
	if err = json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return ok, nil, err
	}
	if s, ok := data["error"].(string); ok && len(s) > 0 {
		return false, nil, fmt.Errorf("Login: error response: %+v", s)
	}

	loginData := data["login"].(map[string]interface{})
	if s, ok := loginData["result"].(string); ok && s != "Success" {
		err = fmt.Errorf("Login: error response: %+v", data)
		return false, nil, err
	}

	return true, resp.Cookies(), err
}

// Purge cleans the cache for specified pages.
func Purge(mwURI string, pageTitles ...string) error {
	u := fmt.Sprintf("%s/api.php?action=purge&format=json&titles=%s", mwURI, strings.Join(pageTitles, "|"))

	req, err := http.NewRequest("POST", u, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data := struct {
		Purge []struct {
			Ns     int
			Purged string
			Title  string
		}
		Error map[string]interface{}
	}{}

	if err = json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return err
	}

	// some heuristic check
	if len(data.Purge) != len(pageTitles) {
		return fmt.Errorf("probably, there's an error: %+v", data)
	}

	return err
}

// GetCategoryMembers returns a list of users who belong to the specified category.
func GetCategoryMembers(mwURI, category string) ([]string, error) {
	// getting users with the category
	u := fmt.Sprintf("%s/api.php?action=query&format=json&list=categorymembers&cmtitle=Category:%s",
		strings.TrimRight(mwURI, "/"), category)
	resp, err := http.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data := struct {
		Query struct {
			Members []struct {
				Title string `json:"title"`
			} `json:"categorymembers"`
		} `json:"query"`
	}{}
	if err = json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decoding failed, HTTP Response Status: %v, URI: %v, error: %v", resp.Status, u, err)
	}

	titles := make([]string, len(data.Query.Members))
	for i := range data.Query.Members {
		titles[i] = data.Query.Members[i].Title
	}

	return titles, err
}

// GetExternalLinks returns a list of external links used on a page.
func GetExternalLinks(mwURI, pageTitle string) ([]string, error) {
	u := fmt.Sprintf("%s/api.php?action=parse&format=json&page=%s&prop=externallinks", mwURI, pageTitle)
	resp, err := http.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data := struct {
		Parse struct {
			Title         string
			ExternalLinks []string
		}
	}{}
	if err = json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return data.Parse.ExternalLinks, nil
}

// GetUsers returns a list of all users.
func GetUsers(mwURI string) ([]string, error) {
	params := url.Values{}
	params.Set("action", "query")
	params.Set("format", "json")
	params.Set("list", "allusers")
	params.Set("meta", "userinfo")
	params.Set("auexcludegroup", "bot")
	params.Set("aulimit", "max")

	data, err := Get(mwURI, params.Encode())
	if err != nil {
		return nil, fmt.Errorf("GetUser: get failed, err: %v, uri: %s, data: %v", err, mwURI, params)
	}

	query, ok := data["query"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("GetUsers: unexpected query type: %+v", data)
	}

	users, ok := query["allusers"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("GetUsers: unexpected allusers type: %+v", query)
	}

	userTitles := make([]string, len(users))
	for i := range users {
		user, ok := users[i].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("GetUsers: unexpected user type: %+v", users[i])
		}

		userTitles[i], ok = user["name"].(string)
		if !ok {
			return nil, fmt.Errorf("GetUsers: unexpected user name type: %+v", users[i])
		}
	}

	return userTitles, nil
}

// Get requests from mediawiki by the uri with the provided parameters.
func Get(mwURI, params string) (map[string]interface{}, error) {
	u := fmt.Sprintf("%s/api.php?%s", mwURI, params)
	resp, err := http.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode > 200 {
		return nil, fmt.Errorf("bad HTTP status")
	}

	data := map[string]interface{}{}

	if err = json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("Get: %v", err)
	}

	return data, nil
}

// Post send POST requests to mediawiki provided cookies returned from Login().
func Post(mwURI, body string, loginCookies []*http.Cookie) (map[string]interface{}, error) {
	u := fmt.Sprintf("%s/api.php", mwURI)

	req, err := http.NewRequest("POST", u, strings.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Cookie", joinCookies(loginCookies))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode > 200 {
		return nil, fmt.Errorf("bad HTTP status")
	}

	data := map[string]interface{}{}

	if err = json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("Post: %v", err)
	}

	return data, nil
}

func joinCookies(cookies []*http.Cookie) string {
	cookieStrings := []string{}
	for _, c := range cookies {
		cookieStrings = append(cookieStrings, c.String())
	}
	return strings.Join(cookieStrings, ";")
}
