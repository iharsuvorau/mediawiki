package mediawiki

import (
	"fmt"
	"testing"
)

func TestLogin(t *testing.T) {
	var err error

	const (
		baseURI   = "http://hefty.local/~ihar/ims/1.32.2"
		loginName = "Ihar@mw-publications"
		loginPass = "71b1nbj468uvp9fq9urctumi2qn37778"
	)

	ok, cookies, err := Login(baseURI, loginName, loginPass)
	if err != nil {
		t.Error(err)
	}
	if !ok {
		t.Error("login failed")
	}
	if len(cookies) == 0 {
		t.Error("cookies must no be empty")
	}
}

func TestUpdatePage(t *testing.T) {
	const (
		mwURI        = "http://hefty.local/~ihar/ims/1.32.2"
		userTitle    = "User:Ihar"
		markup       = "hello from go bot"
		contentModel = "wikitext"
		sectionTitle = "Publications"
		loginName    = "Ihar@mw-publications"
		loginPass    = "71b1nbj468uvp9fq9urctumi2qn37778"
	)
	_, err := UpdatePage(mwURI, userTitle, markup, contentModel, loginName, loginPass, sectionTitle)
	if err != nil {
		t.Error(err)
	}
}

func TestPurge(t *testing.T) {
	type args struct {
		mwURI      string
		pageTitles []string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "A",
			args: args{
				mwURI:      "http://hefty.local/~ihar/ims/1.32.2",
				pageTitles: []string{"Publications"},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Purge(tt.args.mwURI, tt.args.pageTitles...); (err != nil) != tt.wantErr {
				t.Errorf("mwPurge() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetExternalLinks(t *testing.T) {
	const uri = "http://hefty.local/~ihar/ims/1.32.2"

	args := []struct {
		userTitle string
		zeroLinks bool
	}{
		{
			userTitle: "Alvo",
			zeroLinks: false,
		},
		{
			userTitle: "Ihar",
			zeroLinks: false,
		},
		{
			userTitle: "Alan.tkaczyk",
			zeroLinks: true,
		},
		{
			userTitle: "nosuchuser",
			zeroLinks: true,
		},
		{
			userTitle: "",
			zeroLinks: true,
		},
	}

	for _, arg := range args {
		links, err := GetExternalLinks(uri, fmt.Sprintf("User:%s", arg.userTitle))
		if err != nil {
			t.Error(err)
		}
		if len(links) == 0 && !arg.zeroLinks {
			t.Errorf("expect more links")
		}
	}
}

func TestGetUsers(t *testing.T) {
	users, err := GetUsers("http://hefty.local/~ihar/ims/1.32.2")
	t.Logf("users len: %v", len(users))
	if err != nil {
		t.Error(err)
	}
	if len(users) == 0 {
		t.Errorf("expect more users, got %v", users)
	}
}
