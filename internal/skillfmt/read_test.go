package skillfmt

import "testing"

func TestSplitSkillMD(t *testing.T) {
	cases := []struct {
		name, in, body, desc string
	}{
		{"plain", "---\nname: a\ndescription: d\n---\nbody\n", "body\n", "d"},
		{"crlf and bom", "\xef\xbb\xbf---\r\nname: a\r\ndescription: d\r\n---\r\nbody\r\n", "body\n", "d"},
		{"header set aside", HeaderOpen + "\nx: y\n-->\n\n---\nname: a\ndescription: d\n---\n\nbody\n", "\nbody\n", "d"},
		{"closing line is the last line", "---\nname: a\ndescription: d\n---", "", "d"},
		{"empty front matter body", "---\n---\ntext\n", "text\n", ""},
	}
	for _, c := range cases {
		doc, body, vs := SplitSkillMD([]byte(c.in))
		if vs != nil {
			t.Errorf("%s: refused %v", c.name, vs)
			continue
		}
		if body != c.body {
			t.Errorf("%s: body = %q, want %q", c.name, body, c.body)
		}
		if got, _ := doc["description"].(string); got != c.desc {
			t.Errorf("%s: description = %q, want %q", c.name, got, c.desc)
		}
	}
	for _, in := range []string{"no front matter\n", "---\nname: a\n", "---\n- a\n---\n"} {
		if _, _, vs := SplitSkillMD([]byte(in)); vs == nil {
			t.Errorf("%q was read", in)
		}
	}
}
