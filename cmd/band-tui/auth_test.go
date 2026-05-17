package main

import "testing"

func TestExtractMattermostToken(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "raw", in: "abc123\n", want: "abc123"},
		{name: "cookie", in: "MMUSERID=u; MMAUTHTOKEN=tok123; MMCSRF=x", want: "tok123"},
		{name: "cookie header", in: "Cookie: MMUSERID=u; MMAUTHTOKEN=tok123", want: "tok123"},
		{name: "quoted", in: "MMAUTHTOKEN=\"tok123\"", want: "tok123"},
		{name: "missing", in: "MMUSERID=u; MMCSRF=x", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractMattermostToken(tt.in); got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestBuildAuthURL(t *testing.T) {
	got := buildAuthURL("https://band.wb.ru/", "gitlab")
	want := "https://band.wb.ru/oauth/gitlab/login?redirect_to=%2F"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
